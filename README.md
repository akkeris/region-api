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

## Running

### Environment

Set the following environment variables, if this is first time running it see the Setup section.

* PITDB - A postgres database url formatted as postgres://user:pass@host:port/db
* REGION - The region this region-api is running in, this should match the cloud providers definition of "region", e.g., us-west-2 for AWS Oregon Region.
* IMAGE_PULL_SECRET - The path in vault for credentials when pulling images from the registry, if not needed, leave blank, see Image Pull Secret Format section for information how this should be formatted in vault.
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
* F5_SECRET - The path to the token or password in vault
* F5_URL - The https URL of the F5 
* F5_UNIPOOL - When using the F5 in an ingress instruct it to use a single pool. See F5 Setup below for more information.
* F5_CIPHER_LIST - The cipher list to use when creating TLS (SSL) client profiles, defaults to `!SSLv2:!SSLv3:!MD5:!EXPORT:!RSA+3DES:!RSA+RC4:!ECDHE+RC4:!ECDHE+3DES:ECDHE+AES:RSA+AES`
* ISTIO_DOWNPAGE - The maintenance page to use, this should be the service host for an app in akkeris. Defaults to `akkeris404.akkeris-system.svc.cluster.local`.

If the apps or sites ingres uses an F5, the `F5_SECRET` and `F5_URL` should be set.  If using istio these may be left blank.

**Broker Settings**

* SERVICES - A comma delimited list of urls for open service brokers to use e.g., (https://user:pass@hostname/,https://:token@hostname/)
* RABBITMQ_BROKER_URL - todo, get brokers to register with alamo-api, otherwise this is the host of the broker
* MONGODB_BROKER_URL - todo, get brokers to register with alamo-api, otherwise this is the host of the broker
* INFLUXDB_URL - The influx DB url to retrieve http metrics and other custom app data
* PROMETHEUS_URL - The prometheus DB url to retrieve metrics pod data for apps
* NEPTUNE_BROKER_URL - todo, get brokers to register with alamo-api, otherwise this is the host of the broker
* INFLUXDB_BROKER_URL - influx database broker
* CASSANDRA_BROKER_URL - cassandra database broker
* KAFKA_BROKERS - The kafka brokers for this region

**Vault Settings**

* VAULT_ADDR - The https url for vault
* VAULT_TOKEN - The vault token
* VAULT_CERT_STORAGE - Temporary vault path where uncommited certificates may be stored. 
* VAULT_PREFIX - The prefix to use for vault credentials injected as enviornment variables.
* SECRETS - A comma delimited list of vault paths where shared credentials are stored

**DigiCert Certificate Issuer**

Note, if you're using digicert setting `DIGICERT_SECRET` will immediately pick DigiCert as the issuer type, and ignore certificate manager issuer settings.

* DIGICERT_SECRET - The vault path for the digicert token/login secret
* DIGICERT_REQUEST_URL - The digicert request url API
* DIGICERT_URL - The digicert url https://www.digicert.com/services/v2 
* CERT_EMAIL - DigiCert - email address used when requesting new certificates
* CERT_COUNTRY -  DigiCert - the two letter country code used when requesting new certificates (e.g., US), all caps
* CERT_PROVINCE -  DigiCert - the state or province used when requesting new certificates
* CERT_LOCALITY -  DigiCert - the city or locality used when requested new certificates
* CERT_ORG -  DigiCert - the organization or company name used when requesting new certificates
* CERT_ORG_UNIT -  DigiCert - the organization unit or department name used when requesting new certificates
* CERT_VALIDITY_YEARS=2 - The length of time certificates issued will last (in years).

**Cert Manager Certificate Issuer**

This uses jetstack's cert-manager (if installed) to issue certificates. Note above, you may only have the settings for digicert or cert manager set.  For example setting `DIGICERT_SECRET` will automatically pick it as the certificate issuer and these settings will be ignored.

* CERTMANAGER_ISSUER_NAME - The cluster issuer name to use when requesting certificates. The issuer must be a cluster wide issuer.
* CERTMANAGER_PROVIDER_NAME - The provider name to use when requesting certificates, this should match at least one provider in the cluster-wide issuer.
* CERT_NAMESPACE - The namespace to store certificates.  This defaults to `istio-system` to make the certificates (and their secrets) mountable by istio. This has no affect on DigiCert Certificate Issuer as it stores its certificates in vault.

**Optional Environment Variables:**

* DEFAULT_STACK=ds1 - the name of the default stack. If none is specified it assumes the name ds1.
* REVISION_HISTORY_LIMIT=10 - the amount of revisions to keep in replica sets
* LOGSHUTTLE_SERVICE_HOST, LOGSHUTTLE_SERVICE_PORT - where to find the logshuttle
* LOGSESSION_SERVICE_HOST, LOGSESSION_SERVICE_PORT - where to find the logsession
* DOMAIN_BLACKLIST - a comma delimited list of domains or regular expressions that should NOT be in the control of akkeris (region-api), this can be the provider id or domain name (provider id in aws is the hosted zone)
* SUBSCRIPTION_URL
* FEATURE_DEFAULT_OCTHC - a true or false value to enable octhc feature
* FEATURE_DEFAULT_OPSGENIE - a true or false value to enable opsgenie feature
* PUBLIC_DNS_RESOLVER - Used to see if DNS records are already set, this is used incase region api is in a VPN/VPC network. Defaults to 8.8.8.8

**Debugging Environment Variables:**

* DEBUG_K8S - print out all k8s calls, set to true
* DEBUG_F5 - print out all f5 calls, set to true
* MARTINI_ENV

Note that dependencies are managed via `dep` command line tool, run `dep ensure` before building.

```sh
$ docker build -t region-api .
$ docker run -p 3000:3000 -e <see below> region-api
```

### Ingress Formats

f5://ip-address-of-ingress/partition/vip
istio://host-of-ingress/namespace/ingress-gateway-name (its `istio` label on the deployment)

### Image Pull Secret Formats

 Image pull secrets should be held in vault (with the path in vault saved into the environment variable). The secret should have one key called "base64" which contains the base64 encoded JSON objecet in the format of `{"auths":{"registry.hostname.com":{"auth":"...base64authinfo..."},"email":""}}` the "auth" field should have a base64 encoded username:password. The other key should be "name" which contains the name of the secret, usually in the format of `registry.hostname.com-user`. See https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/#log-in-to-docker for an example of the JSON structure that should be base64 encoded and stored in the `base64` field.

### F5 Setup

***Pre-requisetes***

You must have two partitions with two virtual devices on each partition. One for inside and one for outside. The virtual devices are for a application vs site ingresses (e.g, myapp.app.example.io and www.example.io). Each VIP should be configured with the following TLS cipher rules `!SSLv2:!SSLv3:!MD5:!EXPORT:!RSA+3DES:!RSA+RC4:!ECDHE+RC4:!ECDHE+3DES:ECDHE+AES:RSA+AES`.  They should be setup as SNI with a healthy default. Once established configrue the node-watcher and service-watcher apps then continue on to the unipool vs multirule setup below.

*** Unipool vs. Multipool***

The region api works by updating (and adding) ssl client profiles, irules and pools to the F5.  It can operate in two different modes. The multipool mode or unipool mode. The multipool creates a different pool of nodes + ports for each application, where as the unipool uses one pool of nodes and switches the port dynamically in the irule.  These operating modes can be flipped on and off by setting `F5_UNIPOOL` to any value. The service (and node) watcher app is responsible for creating and updating these pools dynamically, while the region api must update and attach the irules to the necessary VIPs. 

For unipool mode you must have one pool named `unipool` with all nodes (kubernetes worker IP addresses) added to it.  The node watcher (when in unipool mode) will update the nodes in this pool automatically.

For multi-pool (non-unipool) mode, both the service watcher and node watcher must not be set in unipool mode. This will create a single pool for each app. Non-unipool mode is not recommended as it does provide higher isolation and fault tolerance (in theory), in practice it has severly taxed the CPU and memory resources of an F5 load balancer.

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





