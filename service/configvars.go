package service

import (
	"database/sql"
	"fmt"
	structs "region-api/structs"
	vault "region-api/vault"
)

func GetServiceConfigVars(db *sql.DB, appname string, space string, appbindings []structs.Bindspec) (error, []structs.EnvVar) {
	elist := []structs.EnvVar{}
	for _, element := range appbindings {
		servicetype := element.Bindtype
		servicename := element.Bindname
		servicevars := []structs.EnvVar{}		

		if servicetype == "redis" {
			err, vars := Getredisvars(servicename)
			if err != nil {
				return err, elist
			}
			for key, value := range vars {
				servicevars = append(servicevars, structs.EnvVar{Name: key, Value: value.(string)})
			}
		} else if servicetype == "memcached" {
			err, vars := Getmemcachedvars(servicename)
			if err != nil {
				return err, elist
			}
			for key, value := range vars {
				servicevars = append(servicevars, structs.EnvVar{Name: key, Value: value.(string)})
			}
		} else if servicetype == "postgres" {
			err, vars := GetPostgresVarsV2(servicename)
			if err != nil {
				return err, elist
			}
			for key, value := range vars {
				servicevars = append(servicevars, structs.EnvVar{Name: key, Value: value.(string)})
			}
		} else if servicetype == "postgresonprem" {
			err, vars := GetPostgresonpremVarsV1(servicename)
			if err != nil {
				return err, elist
			}
			for key, value := range vars {
				servicevars = append(servicevars, structs.EnvVar{Name: key, Value: value.(string)})
			}
		} else if servicetype == "auroramysql" {
			err, vars := Getauroramysqlvars(servicename)
			if err != nil {
				return err, elist
			}
			for key, value := range vars {
				servicevars = append(servicevars, structs.EnvVar{Name: key, Value: value.(string)})
			}
		} else if servicetype == "rabbitmq" {
			err, vars := Getrabbitmqvars(servicename)
			if err != nil {
				return err, elist
			}
			for key, value := range vars {
				servicevars = append(servicevars, structs.EnvVar{Name: key, Value: value.(string)})
			}
		} else if servicetype == "s3" {
			err, vars := Gets3vars(servicename)
			if err != nil {
				return err, elist
			}
			for key, value := range vars {
				servicevars = append(servicevars, structs.EnvVar{Name: key, Value: value.(string)})
			}
		} else if servicetype == "es" {
			err, vars := Getesvars(servicename)
			if err != nil {
				return err, elist
			}
			for key, value := range vars {
				servicevars = append(servicevars, structs.EnvVar{Name: key, Value: value.(string)})
			}
		} else if servicetype == "mongodb" {
			err, vars := Getmongodbvars(servicename)
			if err != nil {
				return err, elist
			}
			for key, value := range vars {
				servicevars = append(servicevars, structs.EnvVar{Name: key, Value: value})
			}
		} else if servicetype == "kafka" {
			err, vars := Getkafkavars(db, appname, space)
			if err != nil {
				return err, elist
			}
			for key, value := range vars {
				servicevars = append(servicevars, structs.EnvVar{Name: key, Value: value.(string)})
			}
		} else if servicetype == "vault" {
			// vault panics if we cannot reach it, just crash the entire API (apparently)
			vars := vault.GetVaultVariables(servicename)
			for _, value := range vars {
				servicevars = append(servicevars, structs.EnvVar{Name: value.Key, Value: value.Value})
			}
                } else if servicetype == "influxdb" {
                        vars,err := GetInfluxdbVars(servicename)
                        if err != nil {
                                return err, elist
                        }
                        for key, value := range vars {
                                servicevars = append(servicevars, structs.EnvVar{Name: key, Value: value.(string)})
                        }
                } else if servicetype == "cassandra" {
                        vars,err := GetCassandraVars(servicename)
                        if err != nil {
                                return err, elist
                        }
                        for key, value := range vars {
                                servicevars = append(servicevars, structs.EnvVar{Name: key, Value: value.(string)})
                        }
		} else if servicetype == "neptune" {
			vars, err := GetNeptuneVars(servicename)
			if err != nil {
				return err, elist
			}
			for key, value := range vars {
				servicevars = append(servicevars, structs.EnvVar{Name: key, Value: value.(string)})
			}

		// if nothing else matches see if we match an
		// open service broker that dynamically registered.
		} else if IsOSBService(servicetype) {
			vars, err := GetOSBBindingCredentials(servicetype, servicename, appname + "-" + space)
			if err != nil {
				return err, elist
			}
			for key, value := range vars {
				servicevars = append(servicevars, structs.EnvVar{Name: key, Value: value.(string)})
			}
		}

		newvars := []structs.EnvVar{}
		removevars := []structs.EnvVar{}

		for _, servicevar := range servicevars {
			rows, err := db.Query("select cvm.action, cvm.varname, cvm.newname from configvarsmap cvm where (cvm.appname || '-' || cvm.space) in (select ab.bindname from appbindings ab where ab.space=$1 and ab.appname=$2 and ab.bindtype='config') and cvm.bindtype=$3 and cvm.bindname=$4", space, appname, servicetype, servicename)
			if err != nil {
				return err, elist
			}
			for rows.Next() {
				var action string
				var varname string
				var newname string
				err = rows.Scan(&action, &varname, &newname)
				if err != nil {
					rows.Close()
					return err, elist
				}
				if varname == servicevar.Name && action == "copy" {
					newvars = append(newvars, structs.EnvVar{Name: newname, Value: servicevar.Value})
				} else if varname == servicevar.Name && action == "delete" {
					removevars = append(removevars, structs.EnvVar{Name: servicevar.Name, Value: servicevar.Value})
				} else if (varname == servicevar.Name || varname == "*") && action == "rename" {
					removevars = append(removevars, structs.EnvVar{Name: servicevar.Name, Value: servicevar.Value})
					newvars = append(newvars, structs.EnvVar{Name: newname, Value: servicevar.Value})
				} else {
					rows.Close()
					return fmt.Errorf("Invalid command in config var %s", action), elist
				}
			}
			rows.Close()
		}

		for _, svar := range servicevars {
			var found = false
			for _, rvar := range removevars {
				if rvar.Name == svar.Name {
					found = true
				}
			}
			if found == false {
				elist = append(elist, svar)
			}
		}
		elist = append(elist, newvars...)
	}
	return nil, elist
}
