# Akkeris Region API

HTTP API for regional akkeris actions, it integrates with kubernetes, and service brokers in addition to the ingress.

[![CircleCI](https://circleci.com/gh/akkeris/region-api.svg?style=svg)](https://circleci.com/gh/akkeris/region-api)

## Running

### Required Resources 

**Kubernetes**

The region api requires a kubernetes runtime. When running in a kubernetes cluster that it administrates, a service account must be specified in its deployment manifest. The service account should have the ability to read, write and delete services, deployments, configmaps, virtual services, certificates, gateways, secrets (within istio-system only), jobs and pods in all namespaces (unless otherwise specified). 

When running the region api out-of-cluster (i.e. locally), a kubeconfig file can be specified using the `--kubeconfig` flag. This will instruct the region api to use the current context in the provided kubeconfig file. 

If no service account or `--kubeconfig` is specified, the region api will use the current context in `$HOME/.kube/config` by default. 

**Postgres Database**

The region api requires a postgres 9.6+ database specified with the `PITDB` environment variable.

**AWS Account**


### Required Environment Variables

Set the following environment variables, if this is first time running it see the Setup section.

* PITDB - A postgres database url formatted as postgres://user:pass@host:port/db
* AWS_ACCESS_KEY_ID - Acceess key id for AWS
* AWS_SECRET_ACCESS_KEY - Access secret for AWS
* REGION - The region this region-api is running in, this should match the cloud providers definition of "region", e.g., us-west-2 for AWS Oregon Region.
* IMAGE_PULL_SECRET - The name of the secret to use when pulling images from the registry. Leave blank if the docker repository is public, defaults to "".
* ENABLE_AUTH - true or false value, set to false for tests
* ALAMO_API_AUTH_SECRET - If ENABLE_AUTH is set to true, this is the path in vault to find the secret, if not needed, leave blank.
* INTERNAL_DOMAIN - The internal domain e.g.., internalapps.example.com
* EXTERNAL_DOMAIN - The internal domain e.g.., apps.example.com
* ALAMO_URL_TEMPLATE - The template for external/public urls https://{name}-{space}.apps.example.com/
* ALAMO_INTERNAL_URL_TEMPLATE - The template for internal/private urls https://{name}-{space}.internalapps.example.com/
* NAGIOS_ADDRESS - The host to nagios to pull information on health of services.

**Ingress Settings**

* APPS_PUBLIC_INTERNAL=(see ingress format)
* APPS_PUBLIC_EXTERNAL=(see ingress format)
* APPS_PRIVATE_INTERNAL=(see ingress format)
* SITES_PUBLIC_INTERNAL=(see ingress format)
* SITES_PUBLIC_EXTERNAL=(see ingress format)
* SITES_PRIVATE_INTERNAL=(see ingress format)
* ISTIO_DOWNPAGE - The maintenance page to use, this should be the service host for an app in akkeris. Defaults to `akkeris404.akkeris-system.svc.cluster.local`

**Broker Settings**

* SERVICES - A comma delimited list of urls for open service brokers to use e.g., (https://user:pass@hostname/,https://:token@hostname/)
* RABBITMQ_BROKER_URL - this is the host of the broker
* MONGODB_BROKER_URL - this is the host of the broker
* INFLUXDB_URL - The influx DB url to retrieve http metrics and other custom app data
* PROMETHEUS_URL - The prometheus DB url to retrieve metrics pod data for apps
* INFLUXDB_BROKER_URL - influx database broker
* CASSANDRA_BROKER_URL - cassandra database broker
* KAFKA_BROKERS - The kafka brokers for this region

**Vault Settings**

* VAULT_ADDR - The https url for vault
* VAULT_TOKEN - The vault token
* VAULT_CERT_STORAGE - Temporary vault path where uncommited certificates may be stored. 
* VAULT_PREFIX - The prefix to use for vault credentials injected as enviornment variables.
* SECRETS - A comma delimited list of vault paths where shared credentials are stored

**Cert Manager Certificate Issuer**

This uses jetstack's cert-manager (if installed) to issue certificates. By default this is the only issuer manager that's supported. 

* DEFAULT_ISSUER - The clusterissuer to use by default when ordering a new certificate (one may be specified when ordering a cert). Defaults to `letsencrypt`.
* CERT_NAMESPACE - The namespace to store certificates.  This defaults to `istio-system` to make the certificates (and their secrets) mountable by istio. 

**Optional Environment Variables:**

* DEFAULT_STACK=ds1 - the name of the default stack. If none is specified it assumes the name ds1.
* REVISION_HISTORY_LIMIT=10 - the amount of revisions to keep in replica sets
* LOGSHUTTLE_SERVICE_HOST, LOGSHUTTLE_SERVICE_PORT - where to find the logshuttle
* LOGSESSION_SERVICE_HOST, LOGSESSION_SERVICE_PORT - where to find the logsession
* DOMAIN_BLACKLIST - a comma delimited list of domains or regular expressions that should NOT be in the control of akkeris (region-api), this can be the provider id or domain name (provider id in aws is the hosted zone)
* PUBLIC_DNS_RESOLVER - Used to see if DNS records are already set, this is used incase region api is in a VPN/VPC network. Defaults to 8.8.8.8

**Debugging Environment Variables:**

* DEBUG_K8S - print out all k8s calls, set to true
* INGRESS_DEBUG - print out debug information on ingress, set to true
* MARTINI_ENV - Always set this to `production`. See https://github.com/go-martini/martini#martini-env

Note that dependencies are managed via `dep` command line tool, run `dep ensure` before building.

```sh
$ docker build -t region-api .
$ docker run -p 3000:3000 -e <see below> region-api
```

### Ingress Formats
Only istio is supported for ingresses currently. All of the Ingress Setting site/app environment variables should follow this format:

```
istio://host-of-ingress/namespace/ingress-gateway-name 
```

The `ingress-gateway-name` is the value of the `istio` label on the istio ingress deployment. The `namespace` is the namespace where istio is deployed (usually istio-system). The `host-of-ingress` must either by a qualified domain name or ip address, this is used to assign the IP address via DNS or a cname record if its a hostname. This should be the user-facing ip address or hostname you want new sites to resolve to, not an internal clusterip or service or node IP.

## Testing

```sh
go test -v ./app
go test -v ./certs
go test -v ./config
go test -v ./router
go test -v ./server
go test -v ./service
go test -v ./space
go test -v ./templates
go test -v ./vault
```

### Database Issues

You must first have `gotest`, `default` and `deck1` space created for the tests to complete properly.  Run

```
INSERT INTO public.spaces (name, internal, compliancetags, stack) VALUES ('gotest', false, null, 'ds1');
INSERT INTO public.spaces (name, internal, compliancetags, stack) VALUES ('deck1', DEFAULT, 'NULL', 'ds1');
INSERT INTO public.spaces (name, internal, compliancetags, stack) VALUES ('default', DEFAULT, 'NULL', 'ds1');
```

And then add some test data for config vars needed here:

```
INSERT INTO public.sets (setid, name, type) VALUES ('48594a81-fa86-4a2e-5284-6aa9f87d319f', 'oct-apitest-cs', 'config');
INSERT INTO public.configvars (setname, varname, varvalue) VALUES ('oct-apitest-cs', 'testvar', 'testvalue');
INSERT INTO public.configvars (setname, varname, varvalue) VALUES ('oct-apitest-cs', 'testvar2', 'testval2');
INSERT INTO public.configvars (setname, varname, varvalue) VALUES ('oct-apitest-cs', 'METADATA_URL', 'http://169.254.169.254/latest/meta-data/placement/availability-zone');
```

You may need to replace ds1 with your stack listed in the `stacks` table.

If your tests are consistently failing run the following on your database:

```
delete from apps where name = 'gotest';
delete from spacesapps where appname = 'gotest';
```
