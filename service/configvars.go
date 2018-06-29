package service

import (
	vault "../vault"
	structs "../structs"
	"strings"
)

func GetServiceConfigVars(appbindings []string) (error, []structs.EnvVar) {
	elist := []structs.EnvVar{}
	for _, element := range appbindings {
		servicetype := strings.Split(element, ":")[0]
		servicename := strings.Split(element, ":")[1]

		if servicetype == "redis" {
			err, vars := Getredisvars(servicename)
			if err != nil {
				return err, elist
			}
			for key, value := range vars {
				elist = append(elist, structs.EnvVar{Name:key, Value:value.(string)})
			}
		}
		if servicetype == "memcached" {
			err, vars := Getmemcachedvars(servicename)
			if err != nil {
				return err, elist
			}
			for key, value := range vars {
				elist = append(elist, structs.EnvVar{Name:key, Value:value.(string)})
			}
		}
		if servicetype == "postgres" {
			err, vars := GetPostgresVarsV2(servicename)
			if err != nil {
				return err, elist
			}
			for key, value := range vars {
				elist = append(elist, structs.EnvVar{Name:key, Value:value.(string)})
			}
		}
		if servicetype == "postgresonprem" {
			err, vars := GetPostgresonpremVarsV1(servicename)
			if err != nil {
				return err, elist
			}
			for key, value := range vars {
				elist = append(elist, structs.EnvVar{Name:key, Value:value.(string)})
			}
		}
		if servicetype == "auroramysql" {
			err, vars := Getauroramysqlvars(servicename)
			if err != nil {
				return err, elist
			}
			for key, value := range vars {
				elist = append(elist, structs.EnvVar{Name:key, Value:value.(string)})
			}
		}
		if servicetype == "rabbitmq" {
			err, vars := Getrabbitmqvars(servicename)
			if err != nil {
				return err, elist
			}
			for key, value := range vars {
				elist = append(elist, structs.EnvVar{Name:key, Value:value.(string)})
			}
		}
		if servicetype == "s3" {
			err, vars := Gets3vars(servicename)
			if err != nil {
				return err, elist
			}
			for key, value := range vars {
				elist = append(elist, structs.EnvVar{Name:key, Value:value.(string)})
			}
		}
		if servicetype == "es" {
			err, vars := Getesvars(servicename)
			if err != nil {
				return err, elist
			}
			for key, value := range vars {
				elist = append(elist, structs.EnvVar{Name:key, Value:value.(string)})
			}
		}
        if servicetype == "mongodb" {
            err, vars := Getmongodbvars(servicename)
            if err != nil {
				return err, elist
			}
			for key, value := range vars {
				elist = append(elist, structs.EnvVar{Name:key, Value:value})
			}
        }
		if servicetype == "vault" {
			// vault panics if we cannot reach it, just crash the entire API (apparently)
			vars := vault.GetVaultVariables(servicename)
			for _, value := range vars {
				elist = append(elist, structs.EnvVar{Name:value.Key, Value:value.Value})
			}
		}
	}
	return nil, elist
}