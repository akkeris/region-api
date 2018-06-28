package service

import (
	structs "region-api/structs"
	vault "region-api/vault"
	"strings"
)

func GetServiceConfigVars(appbindings []string) []structs.EnvVar {
	elist := []structs.EnvVar{}
	for _, element := range appbindings {
		servicetype := strings.Split(element, ":")[0]
		servicename := strings.Split(element, ":")[1]

		if servicetype == "redis" {
			redisvars := Getredisvars(servicename)
			for k, v := range redisvars {
				var e1 structs.EnvVar
				e1.Name = k
				e1.Value = v.(string)
				elist = append(elist, e1)
			}
		}
		if servicetype == "memcached" {
			memcachedvars := Getmemcachedvars(servicename)
			for k, v := range memcachedvars {
				var e1 structs.EnvVar
				e1.Name = k
				e1.Value = v.(string)
				elist = append(elist, e1)
			}
		}
		if servicetype == "postgres" {
			postgresvars := GetPostgresVarsV2(servicename)
			for k, v := range postgresvars {
				var e1 structs.EnvVar
				e1.Name = k
				e1.Value = v.(string)
				elist = append(elist, e1)
			}
		}
		if servicetype == "postgresonprem" {
			postgresvars := GetPostgresonpremVarsV1(servicename)
			for k, v := range postgresvars {
				var e1 structs.EnvVar
				e1.Name = k
				e1.Value = v.(string)
				elist = append(elist, e1)
			}
		}
		if servicetype == "auroramysql" {
			auroramysqlvars := Getauroramysqlvars(servicename)
			for k, v := range auroramysqlvars {
				var e1 structs.EnvVar
				e1.Name = k
				e1.Value = v.(string)
				elist = append(elist, e1)
			}
		}
		if servicetype == "rabbitmq" {
			rabbitmqvars := Getrabbitmqvars(servicename)
			for k, v := range rabbitmqvars {
				var e1 structs.EnvVar
				e1.Name = k
				e1.Value = v.(string)
				elist = append(elist, e1)
			}
		}
		if servicetype == "s3" {
			s3vars := Gets3vars(servicename)
			for k, v := range s3vars {
				var e1 structs.EnvVar
				e1.Name = k
				e1.Value = v.(string)
				elist = append(elist, e1)
			}
		}
		if servicetype == "es" {
			esvars := Getesvars(servicename)
			for k, v := range esvars {
				var e1 structs.EnvVar
				e1.Name = k
				e1.Value = v.(string)
				elist = append(elist, e1)
			}
		}
		if servicetype == "mongodb" {
			mongodbvars, _ := Getmongodbvars(servicename)

			for k, v := range mongodbvars {
				var e1 structs.EnvVar
				e1.Name = k
				e1.Value = v
				elist = append(elist, e1)
			}
		}
		if servicetype == "vault" {
			vaultvars := vault.GetVaultVariables(servicename)
			for _, element := range vaultvars {
				var e1 structs.EnvVar
				e1.Name = element.Key
				e1.Value = element.Value
				elist = append(elist, e1)
			}
		}
	}
	return elist
}
