# Akkeris Region API

HTTP API for regional akkeris actions, it integrates with kubernetes, and service brokers in addition to the ingress.

## Installation

### Setup

When you first run alamo-api on a brand new database it will create the necessary infrastructure, you should first set the following environment variables, they only need to be set when first ran on an empty database.  Note that these only need to be set on first launch, afterwards they can be removed safely, they will have no affect if ran on a database that's already been populated:

* KUBERNETES_API_SERVER - the hostname of the kubernetes server
* KUBERNETES_API_VERSION - the api version of the kubernetes server (usually v1)
* KUBERNETES_CLIENT_TYPE - either token or cert
* KUBERNETES_CERT_SECRET - the path to the certificate secrets in vault, only applicable if KUBERNETES_CLIENT_TYPE=cert
* KUBERNETES_TOKEN_SECRET - the path to the token secret in vault, only applicable if KUBERNETES_CLIENT_TYPE=token
* KUBERNETES_IMAGE_PULL_SECRET - the path to the image pull secret in vault

Ensure you've set the environment variables in the running section below.

### Running

Set the following environment variables, if this is first time running it see the Setup section.

* REGION - The region this region-api is running in, this should match the cloud providers definition of "region", e.g., us-west-2 for AWS Oregon Region.
* DOMAIN_BLACKLIST - a comma delimited list of domains or regular expressions that should NOT be in the control of akkeris (region-api), this can be the provider id or domain name (provider id in aws is the hosted zone)
* VAULT_ADDR - The https url for vault
* VAULT_TOKEN - The vault token
* VAULT_CERT_STORAGE - Temporary vault path where uncommited certificates may be stored. 
* PITDB - A postgres database url formatted as postgres://user:pass@host:port/db
* QUAY_PULL_SECRET
* NAGIOS_ADDRESS
* SUBSCRIPTION_URL
* SECRETS - A comma delimited list of vault paths where shared credentials are stored
* SERVICES - A comma delimited list of urls for open service brokers to use e.g., (https://user:pass@hostname/,https://:token@hostname/)
* ALAMO_API_AUTH_SECRET
* F5_SECRET - The path to the token or password in vault
* F5_URL - The https URL of the F5 
* F5_PARTITION - The external partition on the F5
* F5_PARTITION_INTERNAL - The internal partition on the F5
* F5_VIRTUAL - Public VIP name on the F5
* F5_VIRTUAL_INTERNAL - Internal VIP name on the F5
* FEATURE_DEFAULT_OCTHC - a true or false value to enable octhc feature
* FEATURE_DEFAULT_OPSGENIE - a true or false value to enable opsgenie feature
* ENABLE_AUTH - true or false value, set to false for tests
* CERT_EMAIL - email address used when requesting new certificates
* CERT_COUNTRY - the two letter country code used when requesting new certificates (e.g., US), all caps
* CERT_PROVINCE - the state or province used when requesting new certificates
* CERT_LOCALITY - the city or locality used when requested new certificates
* CERT_ORG - the organization or company name used when requesting new certificates
* CERT_ORG_UNIT - the organization unit or department name used when requesting new certificates
* DIGICERT_REQUEST_URL - The digicert request url API
* DIGICERT_SECRET - The vault path for the digicert token/login secret
* DIGICERT_URL - The digicert url https://www.digicert.com/services/v2 
* INTERNAL_DOMAIN - The internal domain e.g.., internalapps.example.com
* PRIVATE_SNI_VIP - The public ip address for the F5 on the private network
* PRIVATE_SNI_VIP_INTERNAL - The private ip address for the F5 on the private network
* PUBLIC_SNI_VIP - The public IP address for the F5 on the public network
* ALAMO_INTERNAL_URL_TEMPLATE - The template for internal/private urls https://{name}-{space}.internalapps.example.com/
* ALAMO_URL_TEMPLATE - The template for external/public urls https://{name}-{space}.apps.example.com/
* REDIS_BROKER_URL - todo, get brokers to register with alamo-api, otherwise this is the host of the broker
* MEMCACHED_BROKER_URL - todo, get brokers to register with alamo-api, otherwise this is the host of the broker
* RABBITMQ_BROKER_URL - todo, get brokers to register with alamo-api, otherwise this is the host of the broker
* MONGODB_BROKER_URL - todo, get brokers to register with alamo-api, otherwise this is the host of the broker
* S3_BROKER_URL - todo, get brokers to register with alamo-api, otherwise this is the host of the broker
* ES_BROKER_URL - todo, get brokers to register with alamo-api, otherwise this is the host of the broker
* INFLUXDB_URL - The influx DB url to retrieve http metrics and other custom app data
* PROMETHEUS_URL - The prometheus DB url to retrieve metrics pod data for apps
* NEPTUNE_BROKER_URL - todo, get brokers to register with alamo-api, otherwise this is the host of the broker
* INFLUXDB_BROKER_URL - influx database broker
* CASSANDRA_BROKER_URL - cassandra database broker

**Optional Environment Variables:**

* DEFAULT_STACK=ds1 - the name of the default stack. If none is specified it assumes the name ds1.
* CERT_VALIDITY_YEARS=2 - The length of time certificates issued will last (in years).
* REVISION_HISTORY_LIMIT=10 - the amount of revisions to keep in replica sets
* LOGSHUTTLE_SERVICE_HOST, LOGSHUTTLE_SERVICE_PORT - where to find the logshuttle
* LOGSESSION_SERVICE_HOST, LOGSESSION_SERVICE_PORT - where to find the logsession

**Debugging Environment Variables:**

* DEBUG_K8S - print out all k8s calls, set to true
* DEBUG_F5 - print out all f5 calls, set to true
* MARTINI_ENV

Note that dependencies are managed via `dep` command line tool, run `dep ensure` before building.

```sh
$ docker build -t region-api .
$ docker run -p 3000:3000 -e <see below> region-api
```

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


### Certificate Issues

Golang does not implicitly trust intermediate certificates, even if they are signed by a root certificate authority it does turst. Sometimes machines may not have the right intermediate certificates installed and you'll see warnings similar to this:

```
Get https://www.digicert.com/services/v2/order/certificate: x509: certificate signed by unknown authority
Get https://www.digicert.com/services/v2/order/certificate: x509: certificate signed by unknown authority
certExists Get https://www.digicert.com/services/v2/order/certificate: x509: certificate signed by unknown authority
2019/01/16 13:27:35 Get https://www.digicert.com/services/v2/order/certificate: x509: certificate signed by unknown authority
```

When this occurs the website being requested (for an API request most likely) is using an intermediate that isn't explicitly trusted by golang.  To fix this download the certificates that issued the certificate that is erroring and add it to the keychain as "Always Trusted" in osx, or on linux into the trusted ca chain. Most of the time these are some sort of intermediate cert from digicert that must be explicitly trusted: https://www.digicert.com/digicert-root-certificates.htm.

You can also try upgrading golang to get the latest/greatest.





