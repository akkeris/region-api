package router

import (
	. "github.com/smartystreets/goconvey/convey"
	kube "k8s.io/api/core/v1"
	"testing"
)

func TestIstio(t *testing.T) {
	var gateway Gateway
	var ngateway *Gateway = nil
	gateway.APIVersion = "networking.istio.io/v1alpha3"
	gateway.Kind = "Gateway"
	gateway.SetName("sites-public")
	gateway.SetNamespace("sites-system")
	gateway.Spec.Selector = make(map[string]string)
	gateway.Spec.Selector["istio"] = "sites-public-ingressgateway"

	Convey("Test internal istio components", t, func(c C) {

		Convey("Gateways should be able to have hosts and servers added", func() {
			var err error
			var dirty bool
			err, dirty, ngateway = AddHostsAndServers("www.example.com", "www-example-com-tls", &gateway)
			So(err, ShouldBeNil)
			So(dirty, ShouldEqual, true)
			So(len(ngateway.Spec.Servers), ShouldEqual, 2)
			So(len(ngateway.Spec.Servers[0].Hosts), ShouldEqual, 1)
			So(ngateway.Spec.Servers[0].Hosts[0], ShouldEqual, "www.example.com")
			So(ngateway.Spec.Servers[0].Port.Name, ShouldEqual, "https-www-example-com-tls")
			So(ngateway.Spec.Servers[0].Port.Number, ShouldEqual, 443)
			So(ngateway.Spec.Servers[0].TLS.CredentialName, ShouldEqual, "www-example-com-tls")
			So(ngateway.Spec.Servers[0].TLS.MinProtocolVersion, ShouldEqual, "TLSV1_2")
			So(ngateway.Spec.Servers[0].TLS.Mode, ShouldEqual, "SIMPLE")
			So(ngateway.Spec.Servers[0].TLS.PrivateKey, ShouldEqual, "/etc/istio/www-example-com-tls/tls.key")
			So(ngateway.Spec.Servers[0].TLS.ServerCertificate, ShouldEqual, "/etc/istio/www-example-com-tls/tls.crt")

			So(len(ngateway.Spec.Servers[1].Hosts), ShouldEqual, 1)
			So(ngateway.Spec.Servers[1].Hosts[0], ShouldEqual, "www.example.com")
			So(ngateway.Spec.Servers[1].Port.Name, ShouldEqual, "http-www-example-com")
			So(ngateway.Spec.Servers[1].Port.Number, ShouldEqual, 80)
			So(ngateway.Spec.Servers[1].Port.Protocol, ShouldEqual, "HTTP")
		})

		Convey("Gateways should be able to have hosts re-added as a no-op", func() {
			var err error
			var dirty bool
			err, dirty, ngateway = AddHostsAndServers("www.example.com", "www-example-com-tls", ngateway)
			So(err, ShouldBeNil)
			So(dirty, ShouldEqual, false)
			So(len(ngateway.Spec.Servers), ShouldEqual, 2)
			So(len(ngateway.Spec.Servers[0].Hosts), ShouldEqual, 1)
			So(ngateway.Spec.Servers[0].Hosts[0], ShouldEqual, "www.example.com")
			So(ngateway.Spec.Servers[0].Port.Name, ShouldEqual, "https-www-example-com-tls")
			So(ngateway.Spec.Servers[0].Port.Number, ShouldEqual, 443)
			So(ngateway.Spec.Servers[0].TLS.CredentialName, ShouldEqual, "www-example-com-tls")
			So(ngateway.Spec.Servers[0].TLS.MinProtocolVersion, ShouldEqual, "TLSV1_2")
			So(ngateway.Spec.Servers[0].TLS.Mode, ShouldEqual, "SIMPLE")
			So(ngateway.Spec.Servers[0].TLS.PrivateKey, ShouldEqual, "/etc/istio/www-example-com-tls/tls.key")
			So(ngateway.Spec.Servers[0].TLS.ServerCertificate, ShouldEqual, "/etc/istio/www-example-com-tls/tls.crt")

			So(len(ngateway.Spec.Servers[1].Hosts), ShouldEqual, 1)
			So(ngateway.Spec.Servers[1].Hosts[0], ShouldEqual, "www.example.com")
			So(ngateway.Spec.Servers[1].Port.Name, ShouldEqual, "http-www-example-com")
			So(ngateway.Spec.Servers[1].Port.Number, ShouldEqual, 80)
			So(ngateway.Spec.Servers[1].Port.Protocol, ShouldEqual, "HTTP")
		})

		Convey("Gateways should be identical (no-op but dirty) if host removed then re-added", func() {
			var err error
			var dirty bool
			err, dirty, ngateway = RemoveHostsAndServers("www.example.com", "www-example-com-tls", ngateway)
			So(err, ShouldBeNil)
			So(dirty, ShouldEqual, true)
			So(len(ngateway.Spec.Servers), ShouldEqual, 0)
			err, dirty, ngateway = AddHostsAndServers("www.example.com", "www-example-com-tls", ngateway)
			So(err, ShouldBeNil)
			So(dirty, ShouldEqual, true)
			So(len(ngateway.Spec.Servers), ShouldEqual, 2)
			So(len(ngateway.Spec.Servers[0].Hosts), ShouldEqual, 1)
			So(ngateway.Spec.Servers[0].Hosts[0], ShouldEqual, "www.example.com")
			So(ngateway.Spec.Servers[0].Port.Name, ShouldEqual, "https-www-example-com-tls")
			So(ngateway.Spec.Servers[0].Port.Number, ShouldEqual, 443)
			So(ngateway.Spec.Servers[0].TLS.CredentialName, ShouldEqual, "www-example-com-tls")
			So(ngateway.Spec.Servers[0].TLS.MinProtocolVersion, ShouldEqual, "TLSV1_2")
			So(ngateway.Spec.Servers[0].TLS.Mode, ShouldEqual, "SIMPLE")
			So(ngateway.Spec.Servers[0].TLS.PrivateKey, ShouldEqual, "/etc/istio/www-example-com-tls/tls.key")
			So(ngateway.Spec.Servers[0].TLS.ServerCertificate, ShouldEqual, "/etc/istio/www-example-com-tls/tls.crt")

			So(len(ngateway.Spec.Servers[1].Hosts), ShouldEqual, 1)
			So(ngateway.Spec.Servers[1].Hosts[0], ShouldEqual, "www.example.com")
			So(ngateway.Spec.Servers[1].Port.Name, ShouldEqual, "http-www-example-com")
			So(ngateway.Spec.Servers[1].Port.Number, ShouldEqual, 80)
			So(ngateway.Spec.Servers[1].Port.Protocol, ShouldEqual, "HTTP")
		})

		Convey("Gateways servers should be able to have multiple hosts, not servers.", func() {
			var err error
			var dirty bool
			err, dirty, ngateway = AddHostsAndServers("www-sans.example.com", "www-example-com-tls", ngateway)
			So(err, ShouldBeNil)
			So(dirty, ShouldEqual, true)
			So(len(ngateway.Spec.Servers), ShouldEqual, 3)
			So(len(ngateway.Spec.Servers[0].Hosts), ShouldEqual, 2)
			So(ngateway.Spec.Servers[0].Hosts[0], ShouldEqual, "www.example.com")
			So(ngateway.Spec.Servers[0].Hosts[1], ShouldEqual, "www-sans.example.com")
			So(ngateway.Spec.Servers[0].Port.Name, ShouldEqual, "https-www-example-com-tls")
			So(ngateway.Spec.Servers[0].Port.Number, ShouldEqual, 443)
			So(ngateway.Spec.Servers[0].TLS.CredentialName, ShouldEqual, "www-example-com-tls")
			So(ngateway.Spec.Servers[0].TLS.MinProtocolVersion, ShouldEqual, "TLSV1_2")
			So(ngateway.Spec.Servers[0].TLS.Mode, ShouldEqual, "SIMPLE")
			So(ngateway.Spec.Servers[0].TLS.PrivateKey, ShouldEqual, "/etc/istio/www-example-com-tls/tls.key")
			So(ngateway.Spec.Servers[0].TLS.ServerCertificate, ShouldEqual, "/etc/istio/www-example-com-tls/tls.crt")

			So(len(ngateway.Spec.Servers[1].Hosts), ShouldEqual, 1)
			So(ngateway.Spec.Servers[1].Hosts[0], ShouldEqual, "www.example.com")
			So(ngateway.Spec.Servers[1].Port.Name, ShouldEqual, "http-www-example-com")
			So(ngateway.Spec.Servers[1].Port.Number, ShouldEqual, 80)
			So(ngateway.Spec.Servers[1].Port.Protocol, ShouldEqual, "HTTP")

			So(len(ngateway.Spec.Servers[2].Hosts), ShouldEqual, 1)
			So(ngateway.Spec.Servers[2].Hosts[0], ShouldEqual, "www-sans.example.com")
			So(ngateway.Spec.Servers[2].Port.Name, ShouldEqual, "http-www-sans-example-com")
			So(ngateway.Spec.Servers[2].Port.Number, ShouldEqual, 80)
			So(ngateway.Spec.Servers[2].Port.Protocol, ShouldEqual, "HTTP")
		})

		Convey("Gateway servers misconfigured should reconfigure themselves.", func() {
			var err error
			var dirty bool
			// clear things out
			err, dirty, ngateway = RemoveHostsAndServers("www.example.com", "www-example-com-tls", ngateway)
			So(err, ShouldBeNil)
			So(dirty, ShouldEqual, true)
			So(len(ngateway.Spec.Servers), ShouldEqual, 2)

			err, dirty, ngateway = RemoveHostsAndServers("www-sans.example.com", "www-example-com-tls", ngateway)
			So(err, ShouldBeNil)
			So(dirty, ShouldEqual, true)
			So(len(ngateway.Spec.Servers), ShouldEqual, 0)

			// accidently assign a www.example.com onto the star wildcard sni default certificate
			err, dirty, ngateway = RemoveHostsAndServers("sni.example.com", "star-certificate", ngateway)
			err, dirty, ngateway = AddHostsAndServers("sni.example.com", "star-certificate", ngateway)
			So(err, ShouldBeNil)
			So(dirty, ShouldEqual, true)
			So(len(ngateway.Spec.Servers), ShouldEqual, 2)

			err, dirty, ngateway = RemoveHostsAndServers("www.example.com", "star-certificate", ngateway)
			err, dirty, ngateway = AddHostsAndServers("www.example.com", "star-certificate", ngateway)
			So(err, ShouldBeNil)
			So(dirty, ShouldEqual, true)
			So(len(ngateway.Spec.Servers), ShouldEqual, 3)

			// try and correct it so its on its own server
			err, dirty, ngateway = RemoveHostsAndServers("www.example.com", "www-example-com-tls", ngateway)
			err, dirty, ngateway = AddHostsAndServers("www.example.com", "www-example-com-tls", ngateway)
			So(err, ShouldBeNil)
			So(dirty, ShouldEqual, true)
			So(len(ngateway.Spec.Servers), ShouldEqual, 4)

			So(len(ngateway.Spec.Servers[0].Hosts), ShouldEqual, 1)
			So(ngateway.Spec.Servers[0].Hosts[0], ShouldEqual, "sni.example.com")
			So(ngateway.Spec.Servers[0].Port.Name, ShouldEqual, "https-star-certificate")
			So(ngateway.Spec.Servers[0].Port.Number, ShouldEqual, 443)
			So(ngateway.Spec.Servers[0].TLS.CredentialName, ShouldEqual, "star-certificate")
			So(ngateway.Spec.Servers[0].TLS.MinProtocolVersion, ShouldEqual, "TLSV1_2")
			So(ngateway.Spec.Servers[0].TLS.Mode, ShouldEqual, "SIMPLE")
			So(ngateway.Spec.Servers[0].TLS.PrivateKey, ShouldEqual, "/etc/istio/star-certificate/tls.key")
			So(ngateway.Spec.Servers[0].TLS.ServerCertificate, ShouldEqual, "/etc/istio/star-certificate/tls.crt")

			So(ngateway.Spec.Servers[1].Hosts[0], ShouldEqual, "sni.example.com")
			So(ngateway.Spec.Servers[1].Port.Name, ShouldEqual, "http-sni-example-com")
			So(ngateway.Spec.Servers[1].Port.Number, ShouldEqual, 80)

			So(len(ngateway.Spec.Servers[2].Hosts), ShouldEqual, 1)
			So(ngateway.Spec.Servers[2].Hosts[0], ShouldEqual, "www.example.com")
			So(ngateway.Spec.Servers[2].Port.Name, ShouldEqual, "https-www-example-com-tls")
			So(ngateway.Spec.Servers[2].Port.Number, ShouldEqual, 443)
			So(ngateway.Spec.Servers[2].TLS.CredentialName, ShouldEqual, "www-example-com-tls")
			So(ngateway.Spec.Servers[2].TLS.MinProtocolVersion, ShouldEqual, "TLSV1_2")
			So(ngateway.Spec.Servers[2].TLS.Mode, ShouldEqual, "SIMPLE")
			So(ngateway.Spec.Servers[2].TLS.PrivateKey, ShouldEqual, "/etc/istio/www-example-com-tls/tls.key")
			So(ngateway.Spec.Servers[2].TLS.ServerCertificate, ShouldEqual, "/etc/istio/www-example-com-tls/tls.crt")

			So(len(ngateway.Spec.Servers[3].Hosts), ShouldEqual, 1)
			So(ngateway.Spec.Servers[3].Hosts[0], ShouldEqual, "www.example.com")
			So(ngateway.Spec.Servers[3].Port.Name, ShouldEqual, "http-www-example-com")
			So(ngateway.Spec.Servers[3].Port.Number, ShouldEqual, 80)
			So(ngateway.Spec.Servers[3].Port.Protocol, ShouldEqual, "HTTP")
		})

		Convey("Ensure the virtual service non-deep, with slash, non-deep and private gateway works", func() {
			vs, err := PrepareVirtualServiceForCreateorUpdate("www.example.com", true, []Route{Route{Domain:"www.example.com",Path:"/",Space:"default",App:"test",ReplacePath:"/",Port:"80"}})
			So(err, ShouldBeNil)
			So(vs.GetName(), ShouldEqual, "www.example.com")
			So(vs.GetNamespace(), ShouldEqual, "sites-system")
			So(len(vs.Spec.Gateways), ShouldEqual, 1)
			So(vs.Spec.Gateways[0], ShouldEqual, "sites-private")
			So(len(vs.Spec.Hosts), ShouldEqual, 1)
			So(vs.Spec.Hosts[0], ShouldEqual, "www.example.com")
			So(len(vs.Spec.HTTP), ShouldEqual, 1)
			So(len(vs.Spec.HTTP[0].Match), ShouldEqual, 1)
			So(vs.Spec.HTTP[0].Match[0].IgnoreUriCase, ShouldEqual, true)
			So(vs.Spec.HTTP[0].Match[0].URI.Prefix, ShouldEqual, "/")
			So(vs.Spec.HTTP[0].Rewrite.URI, ShouldEqual, "/")
			So(len(vs.Spec.HTTP[0].Route), ShouldEqual, 1)
			So(vs.Spec.HTTP[0].Route[0].Destination.Host, ShouldEqual, "test.default.svc.cluster.local")
			So(vs.Spec.HTTP[0].Route[0].Destination.Port.Number, ShouldEqual, 80)
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Forwarded-Path"], ShouldEqual, "/")
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Orig-Host"], ShouldEqual, "www.example.com")
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Orig-Path"], ShouldEqual, "")
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Orig-Port"], ShouldEqual, "443")
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Orig-Proto"], ShouldEqual, "https")
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Request-Start"], ShouldEqual, "t=%START_TIME(%s.%3f)%")
			So(vs.Spec.HTTP[0].Headers.Response.Set["Strict-Transport-Security"], ShouldEqual, "max-age=31536000; includeSubDomains")
		})

		Convey("Ensure virtual service deep, without slash, to non-deep, with slash and public gateway works", func() {
			vs, err := PrepareVirtualServiceForCreateorUpdate("www.example.com", false, []Route{Route{Domain:"www.example.com",Path:"/ui/path/toapp",Space:"default",App:"test",ReplacePath:"/",Port:"80"}})
			So(err, ShouldBeNil)
			So(vs.GetName(), ShouldEqual, "www.example.com")
			So(vs.GetNamespace(), ShouldEqual, "sites-system")
			So(len(vs.Spec.Gateways), ShouldEqual, 1)
			So(vs.Spec.Gateways[0], ShouldEqual, "sites-public")
			So(len(vs.Spec.Hosts), ShouldEqual, 1)
			So(vs.Spec.Hosts[0], ShouldEqual, "www.example.com")
			So(len(vs.Spec.HTTP), ShouldEqual, 2)

			So(len(vs.Spec.HTTP[0].Match), ShouldEqual, 1)
			So(vs.Spec.HTTP[0].Match[0].IgnoreUriCase, ShouldEqual, true)
			So(vs.Spec.HTTP[0].Match[0].URI.Prefix, ShouldEqual, "/ui/path/toapp/")
			So(vs.Spec.HTTP[0].Rewrite.URI, ShouldEqual, "/")
			So(len(vs.Spec.HTTP[0].Route), ShouldEqual, 1)
			So(vs.Spec.HTTP[0].Route[0].Destination.Host, ShouldEqual, "test.default.svc.cluster.local")
			So(vs.Spec.HTTP[0].Route[0].Destination.Port.Number, ShouldEqual, 80)
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Forwarded-Path"], ShouldEqual, "/ui/path/toapp/")
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Orig-Host"], ShouldEqual, "www.example.com")
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Orig-Path"], ShouldEqual, "/ui/path/toapp")
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Orig-Port"], ShouldEqual, "443")
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Orig-Proto"], ShouldEqual, "https")
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Request-Start"], ShouldEqual, "t=%START_TIME(%s.%3f)%")
			So(vs.Spec.HTTP[0].Headers.Response.Set["Strict-Transport-Security"], ShouldEqual, "max-age=31536000; includeSubDomains")

			So(len(vs.Spec.HTTP[1].Match), ShouldEqual, 1)
			So(vs.Spec.HTTP[1].Match[0].IgnoreUriCase, ShouldEqual, true)
			So(vs.Spec.HTTP[1].Match[0].URI.Prefix, ShouldEqual, "/ui/path/toapp")
			So(vs.Spec.HTTP[1].Rewrite.URI, ShouldEqual, "/")
			So(len(vs.Spec.HTTP[1].Route), ShouldEqual, 1)
			So(vs.Spec.HTTP[1].Route[0].Destination.Host, ShouldEqual, "test.default.svc.cluster.local")
			So(vs.Spec.HTTP[1].Route[0].Destination.Port.Number, ShouldEqual, 80)
			So(vs.Spec.HTTP[1].Headers.Request.Set["X-Forwarded-Path"], ShouldEqual, "/ui/path/toapp")
			So(vs.Spec.HTTP[1].Headers.Request.Set["X-Orig-Host"], ShouldEqual, "www.example.com")
			So(vs.Spec.HTTP[1].Headers.Request.Set["X-Orig-Path"], ShouldEqual, "/ui/path/toapp")
			So(vs.Spec.HTTP[1].Headers.Request.Set["X-Orig-Port"], ShouldEqual, "443")
			So(vs.Spec.HTTP[1].Headers.Request.Set["X-Orig-Proto"], ShouldEqual, "https")
			So(vs.Spec.HTTP[1].Headers.Request.Set["X-Request-Start"], ShouldEqual, "t=%START_TIME(%s.%3f)%")
			So(vs.Spec.HTTP[1].Headers.Response.Set["Strict-Transport-Security"], ShouldEqual, "max-age=31536000; includeSubDomains")
		})

		Convey("Ensure virtual service deep, with slash, to non-deep, with slash and public gateway works", func() {
			vs, err := PrepareVirtualServiceForCreateorUpdate("www.example.com", false, []Route{Route{Domain:"www.example.com",Path:"/ui/path/toapp/",Space:"default",App:"test",ReplacePath:"/",Port:"80"}})
			So(err, ShouldBeNil)
			So(vs.GetName(), ShouldEqual, "www.example.com")
			So(vs.GetNamespace(), ShouldEqual, "sites-system")
			So(len(vs.Spec.Gateways), ShouldEqual, 1)
			So(vs.Spec.Gateways[0], ShouldEqual, "sites-public")
			So(len(vs.Spec.Hosts), ShouldEqual, 1)
			So(vs.Spec.Hosts[0], ShouldEqual, "www.example.com")
			So(len(vs.Spec.HTTP), ShouldEqual, 2)

			So(len(vs.Spec.HTTP[0].Match), ShouldEqual, 1)
			So(vs.Spec.HTTP[0].Match[0].IgnoreUriCase, ShouldEqual, true)
			So(vs.Spec.HTTP[0].Match[0].URI.Prefix, ShouldEqual, "/ui/path/toapp/")
			So(vs.Spec.HTTP[0].Rewrite.URI, ShouldEqual, "/")
			So(len(vs.Spec.HTTP[0].Route), ShouldEqual, 1)
			So(vs.Spec.HTTP[0].Route[0].Destination.Host, ShouldEqual, "test.default.svc.cluster.local")
			So(vs.Spec.HTTP[0].Route[0].Destination.Port.Number, ShouldEqual, 80)
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Forwarded-Path"], ShouldEqual, "/ui/path/toapp/")
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Orig-Host"], ShouldEqual, "www.example.com")
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Orig-Path"], ShouldEqual, "/ui/path/toapp")
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Orig-Port"], ShouldEqual, "443")
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Orig-Proto"], ShouldEqual, "https")
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Request-Start"], ShouldEqual, "t=%START_TIME(%s.%3f)%")
			So(vs.Spec.HTTP[0].Headers.Response.Set["Strict-Transport-Security"], ShouldEqual, "max-age=31536000; includeSubDomains")

			So(len(vs.Spec.HTTP[1].Match), ShouldEqual, 1)
			So(vs.Spec.HTTP[1].Match[0].IgnoreUriCase, ShouldEqual, true)
			So(vs.Spec.HTTP[1].Match[0].URI.Prefix, ShouldEqual, "/ui/path/toapp")
			So(vs.Spec.HTTP[1].Rewrite.URI, ShouldEqual, "/")
			So(len(vs.Spec.HTTP[1].Route), ShouldEqual, 1)
			So(vs.Spec.HTTP[1].Route[0].Destination.Host, ShouldEqual, "test.default.svc.cluster.local")
			So(vs.Spec.HTTP[1].Route[0].Destination.Port.Number, ShouldEqual, 80)
			So(vs.Spec.HTTP[1].Headers.Request.Set["X-Forwarded-Path"], ShouldEqual, "/ui/path/toapp")
			So(vs.Spec.HTTP[1].Headers.Request.Set["X-Orig-Host"], ShouldEqual, "www.example.com")
			So(vs.Spec.HTTP[1].Headers.Request.Set["X-Orig-Path"], ShouldEqual, "/ui/path/toapp")
			So(vs.Spec.HTTP[1].Headers.Request.Set["X-Orig-Port"], ShouldEqual, "443")
			So(vs.Spec.HTTP[1].Headers.Request.Set["X-Orig-Proto"], ShouldEqual, "https")
			So(vs.Spec.HTTP[1].Headers.Request.Set["X-Request-Start"], ShouldEqual, "t=%START_TIME(%s.%3f)%")
			So(vs.Spec.HTTP[1].Headers.Response.Set["Strict-Transport-Security"], ShouldEqual, "max-age=31536000; includeSubDomains")
		})

		Convey("Ensure virtual service deep, with slash, to deep, with slash and public gateway works", func() {
			vs, err := PrepareVirtualServiceForCreateorUpdate("www.example.com", false, []Route{Route{Domain:"www.example.com",Path:"/path/toapp/",Space:"default",App:"test",ReplacePath:"/path/toapp/",Port:"80"}})
			So(err, ShouldBeNil)
			So(vs.GetName(), ShouldEqual, "www.example.com")
			So(vs.GetNamespace(), ShouldEqual, "sites-system")
			So(len(vs.Spec.Gateways), ShouldEqual, 1)
			So(vs.Spec.Gateways[0], ShouldEqual, "sites-public")
			So(len(vs.Spec.Hosts), ShouldEqual, 1)
			So(vs.Spec.Hosts[0], ShouldEqual, "www.example.com")
			So(len(vs.Spec.HTTP), ShouldEqual, 2)

			So(len(vs.Spec.HTTP[0].Match), ShouldEqual, 1)
			So(vs.Spec.HTTP[0].Match[0].IgnoreUriCase, ShouldEqual, true)
			So(vs.Spec.HTTP[0].Match[0].URI.Prefix, ShouldEqual, "/path/toapp/")
			So(vs.Spec.HTTP[0].Rewrite.URI, ShouldEqual, "/path/toapp/")
			So(len(vs.Spec.HTTP[0].Route), ShouldEqual, 1)
			So(vs.Spec.HTTP[0].Route[0].Destination.Host, ShouldEqual, "test.default.svc.cluster.local")
			So(vs.Spec.HTTP[0].Route[0].Destination.Port.Number, ShouldEqual, 80)
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Forwarded-Path"], ShouldEqual, "/path/toapp/")
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Orig-Host"], ShouldEqual, "www.example.com")
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Orig-Path"], ShouldEqual, "/path/toapp")
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Orig-Port"], ShouldEqual, "443")
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Orig-Proto"], ShouldEqual, "https")
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Request-Start"], ShouldEqual, "t=%START_TIME(%s.%3f)%")
			So(vs.Spec.HTTP[0].Headers.Response.Set["Strict-Transport-Security"], ShouldEqual, "max-age=31536000; includeSubDomains")

			So(len(vs.Spec.HTTP[1].Match), ShouldEqual, 1)
			So(vs.Spec.HTTP[1].Match[0].IgnoreUriCase, ShouldEqual, true)
			So(vs.Spec.HTTP[1].Match[0].URI.Prefix, ShouldEqual, "/path/toapp")
			So(vs.Spec.HTTP[1].Rewrite.URI, ShouldEqual, "/path/toapp/")
			So(len(vs.Spec.HTTP[1].Route), ShouldEqual, 1)
			So(vs.Spec.HTTP[1].Route[0].Destination.Host, ShouldEqual, "test.default.svc.cluster.local")
			So(vs.Spec.HTTP[1].Route[0].Destination.Port.Number, ShouldEqual, 80)
			So(vs.Spec.HTTP[1].Headers.Request.Set["X-Forwarded-Path"], ShouldEqual, "/path/toapp")
			So(vs.Spec.HTTP[1].Headers.Request.Set["X-Orig-Host"], ShouldEqual, "www.example.com")
			So(vs.Spec.HTTP[1].Headers.Request.Set["X-Orig-Path"], ShouldEqual, "/path/toapp")
			So(vs.Spec.HTTP[1].Headers.Request.Set["X-Orig-Port"], ShouldEqual, "443")
			So(vs.Spec.HTTP[1].Headers.Request.Set["X-Orig-Proto"], ShouldEqual, "https")
			So(vs.Spec.HTTP[1].Headers.Request.Set["X-Request-Start"], ShouldEqual, "t=%START_TIME(%s.%3f)%")
			So(vs.Spec.HTTP[1].Headers.Response.Set["Strict-Transport-Security"], ShouldEqual, "max-age=31536000; includeSubDomains")
		})

		Convey("Ensure virtual service deep, without slash, to deep, without slash and public gateway works", func() {
			vs, err := PrepareVirtualServiceForCreateorUpdate("www.example.com", false, []Route{Route{Domain:"www.example.com",Path:"/path/toapp",Space:"default",App:"test",ReplacePath:"/path/toapp",Port:"80"}})
			So(err, ShouldBeNil)
			So(vs.GetName(), ShouldEqual, "www.example.com")
			So(vs.GetNamespace(), ShouldEqual, "sites-system")
			So(len(vs.Spec.Gateways), ShouldEqual, 1)
			So(vs.Spec.Gateways[0], ShouldEqual, "sites-public")
			So(len(vs.Spec.Hosts), ShouldEqual, 1)
			So(vs.Spec.Hosts[0], ShouldEqual, "www.example.com")
			So(len(vs.Spec.HTTP), ShouldEqual, 2)

			So(len(vs.Spec.HTTP[0].Match), ShouldEqual, 1)
			So(vs.Spec.HTTP[0].Match[0].IgnoreUriCase, ShouldEqual, true)
			So(vs.Spec.HTTP[0].Match[0].URI.Prefix, ShouldEqual, "/path/toapp/")
			So(vs.Spec.HTTP[0].Rewrite.URI, ShouldEqual, "/path/toapp/")
			So(len(vs.Spec.HTTP[0].Route), ShouldEqual, 1)
			So(vs.Spec.HTTP[0].Route[0].Destination.Host, ShouldEqual, "test.default.svc.cluster.local")
			So(vs.Spec.HTTP[0].Route[0].Destination.Port.Number, ShouldEqual, 80)
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Forwarded-Path"], ShouldEqual, "/path/toapp/")
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Orig-Host"], ShouldEqual, "www.example.com")
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Orig-Path"], ShouldEqual, "/path/toapp")
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Orig-Port"], ShouldEqual, "443")
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Orig-Proto"], ShouldEqual, "https")
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Request-Start"], ShouldEqual, "t=%START_TIME(%s.%3f)%")
			So(vs.Spec.HTTP[0].Headers.Response.Set["Strict-Transport-Security"], ShouldEqual, "max-age=31536000; includeSubDomains")

			So(len(vs.Spec.HTTP[1].Match), ShouldEqual, 1)
			So(vs.Spec.HTTP[1].Match[0].IgnoreUriCase, ShouldEqual, true)
			So(vs.Spec.HTTP[1].Match[0].URI.Prefix, ShouldEqual, "/path/toapp")
			So(vs.Spec.HTTP[1].Rewrite.URI, ShouldEqual, "/path/toapp")
			So(len(vs.Spec.HTTP[1].Route), ShouldEqual, 1)
			So(vs.Spec.HTTP[1].Route[0].Destination.Host, ShouldEqual, "test.default.svc.cluster.local")
			So(vs.Spec.HTTP[1].Route[0].Destination.Port.Number, ShouldEqual, 80)
			So(vs.Spec.HTTP[1].Headers.Request.Set["X-Forwarded-Path"], ShouldEqual, "/path/toapp")
			So(vs.Spec.HTTP[1].Headers.Request.Set["X-Orig-Host"], ShouldEqual, "www.example.com")
			So(vs.Spec.HTTP[1].Headers.Request.Set["X-Orig-Path"], ShouldEqual, "/path/toapp")
			So(vs.Spec.HTTP[1].Headers.Request.Set["X-Orig-Port"], ShouldEqual, "443")
			So(vs.Spec.HTTP[1].Headers.Request.Set["X-Orig-Proto"], ShouldEqual, "https")
			So(vs.Spec.HTTP[1].Headers.Request.Set["X-Request-Start"], ShouldEqual, "t=%START_TIME(%s.%3f)%")
			So(vs.Spec.HTTP[1].Headers.Response.Set["Strict-Transport-Security"], ShouldEqual, "max-age=31536000; includeSubDomains")
		})

		Convey("Ensure virtual service deep, without slash, to deep, without slash and public gateway works different bases", func() {
			vs, err := PrepareVirtualServiceForCreateorUpdate("www.example.com", false, []Route{Route{Domain:"www.example.com",Path:"/logout",Space:"default",App:"test",ReplacePath:"/oauth/path/logout",Port:"80"}})
			So(err, ShouldBeNil)
			So(vs.GetName(), ShouldEqual, "www.example.com")
			So(vs.GetNamespace(), ShouldEqual, "sites-system")
			So(len(vs.Spec.Gateways), ShouldEqual, 1)
			So(vs.Spec.Gateways[0], ShouldEqual, "sites-public")
			So(len(vs.Spec.Hosts), ShouldEqual, 1)
			So(vs.Spec.Hosts[0], ShouldEqual, "www.example.com")
			So(len(vs.Spec.HTTP), ShouldEqual, 2)

			So(len(vs.Spec.HTTP[0].Match), ShouldEqual, 1)
			So(vs.Spec.HTTP[0].Match[0].IgnoreUriCase, ShouldEqual, true)
			So(vs.Spec.HTTP[0].Match[0].URI.Prefix, ShouldEqual, "/logout/")
			So(vs.Spec.HTTP[0].Rewrite.URI, ShouldEqual, "/oauth/path/logout/")
			So(len(vs.Spec.HTTP[0].Route), ShouldEqual, 1)
			So(vs.Spec.HTTP[0].Route[0].Destination.Host, ShouldEqual, "test.default.svc.cluster.local")
			So(vs.Spec.HTTP[0].Route[0].Destination.Port.Number, ShouldEqual, 80)
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Forwarded-Path"], ShouldEqual, "/logout/")
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Orig-Host"], ShouldEqual, "www.example.com")
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Orig-Path"], ShouldEqual, "/logout")
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Orig-Port"], ShouldEqual, "443")
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Orig-Proto"], ShouldEqual, "https")
			So(vs.Spec.HTTP[0].Headers.Request.Set["X-Request-Start"], ShouldEqual, "t=%START_TIME(%s.%3f)%")
			So(vs.Spec.HTTP[0].Headers.Response.Set["Strict-Transport-Security"], ShouldEqual, "max-age=31536000; includeSubDomains")

			So(len(vs.Spec.HTTP[1].Match), ShouldEqual, 1)
			So(vs.Spec.HTTP[1].Match[0].IgnoreUriCase, ShouldEqual, true)
			So(vs.Spec.HTTP[1].Match[0].URI.Prefix, ShouldEqual, "/logout")
			So(vs.Spec.HTTP[1].Rewrite.URI, ShouldEqual, "/oauth/path/logout")
			So(len(vs.Spec.HTTP[1].Route), ShouldEqual, 1)
			So(vs.Spec.HTTP[1].Route[0].Destination.Host, ShouldEqual, "test.default.svc.cluster.local")
			So(vs.Spec.HTTP[1].Route[0].Destination.Port.Number, ShouldEqual, 80)
			So(vs.Spec.HTTP[1].Headers.Request.Set["X-Forwarded-Path"], ShouldEqual, "/logout")
			So(vs.Spec.HTTP[1].Headers.Request.Set["X-Orig-Host"], ShouldEqual, "www.example.com")
			So(vs.Spec.HTTP[1].Headers.Request.Set["X-Orig-Path"], ShouldEqual, "/logout")
			So(vs.Spec.HTTP[1].Headers.Request.Set["X-Orig-Port"], ShouldEqual, "443")
			So(vs.Spec.HTTP[1].Headers.Request.Set["X-Orig-Proto"], ShouldEqual, "https")
			So(vs.Spec.HTTP[1].Headers.Request.Set["X-Request-Start"], ShouldEqual, "t=%START_TIME(%s.%3f)%")
			So(vs.Spec.HTTP[1].Headers.Response.Set["Strict-Transport-Security"], ShouldEqual, "max-age=31536000; includeSubDomains")
		})

		Convey("Ensure creating a secret from a certificate works", func() {
			cert := `-----BEGIN CERTIFICATE-----
MIIDtDCCApwCCQD+cauHotiQDjANBgkqhkiG9w0BAQsFADCBmzELMAkGA1UEBhMC
VVMxDTALBgNVBAgMBFV0YWgxFzAVBgNVBAcMDlNhbHQgTGFrZSBDaXR5MRAwDgYD
VQQKDAdBa2tlcmlzMRAwDgYDVQQLDAdUZXN0aW5nMRwwGgYDVQQDDBNha2tlcmlz
LmV4YW1wbGUuY29tMSIwIAYJKoZIhvcNAQkBFhNha2tlcmlzQGV4YW1wbGUuY29t
MB4XDTIwMDExNTE3NTQ0MFoXDTIxMDExNDE3NTQ0MFowgZsxCzAJBgNVBAYTAlVT
MQ0wCwYDVQQIDARVdGFoMRcwFQYDVQQHDA5TYWx0IExha2UgQ2l0eTEQMA4GA1UE
CgwHQWtrZXJpczEQMA4GA1UECwwHVGVzdGluZzEcMBoGA1UEAwwTYWtrZXJpcy5l
eGFtcGxlLmNvbTEiMCAGCSqGSIb3DQEJARYTYWtrZXJpc0BleGFtcGxlLmNvbTCC
ASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAK7YUd7cKmIQ0VaQT8/FbSwy
/hdQRSaXe+Rn8PWh2OMU0OoQ6NfQr3d/jm0oiTI79mRbV2sEv3CG1e2dbGRFtz7P
6K9rgg51tlj/ZglXItktaDG5H2ylvfeuwqtXaZ2fJhrq1vbabWZf5otEISTZDA+F
EMlOCVqtzlI7Wtax0gw5xVe9mW2foTr0ek/pkhbJpYItzWpZZLBR+WfiJutupw74
ss8Y0fTv/mlgvHZf1f+WwD8RH+VH6LZwhT8L0tMerIodL2q2HAo033lHJDbVAgrO
6ZW+WJT75/QjJIdDEDtmyxq1djHAKkWUJwy55lbrkhN76oWZ9neim9Q03K8nPmMC
AwEAATANBgkqhkiG9w0BAQsFAAOCAQEAELMVniZ8aUB8odNHmSuDkZc1WzIotLiD
vM6aLetQKyR826/WATguT4Ap3FTNd1wz7aT3BKRUIY6AzqZ2TR/3/E3QZXFfyWNe
gPKLtOwnniNLSvO+BIM4s60tGw7lvsbXBF0vWKqMeDhQwO7B+ruFyRE4NT8dSVRe
2XNmrkGiGXyAWrll0EsUPNNS3qfunMOcXzPlriekMjc6PeKxTUyec71Lqy8837wf
HtNfMFksbylTsAF1dCahskYy1INyoHj+UOBq+7ddbxgIWcCbycepUfv/f97LS53N
BcndTaegAzDNW0g+0kwIV/uIXg1+WP0ltJCgswETqKApzExN+w4qjQ==
-----END CERTIFICATE-----
`
			key := `-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCu2FHe3CpiENFW
kE/PxW0sMv4XUEUml3vkZ/D1odjjFNDqEOjX0K93f45tKIkyO/ZkW1drBL9whtXt
nWxkRbc+z+iva4IOdbZY/2YJVyLZLWgxuR9spb33rsKrV2mdnyYa6tb22m1mX+aL
RCEk2QwPhRDJTglarc5SO1rWsdIMOcVXvZltn6E69HpP6ZIWyaWCLc1qWWSwUfln
4ibrbqcO+LLPGNH07/5pYLx2X9X/lsA/ER/lR+i2cIU/C9LTHqyKHS9qthwKNN95
RyQ21QIKzumVvliU++f0IySHQxA7ZssatXYxwCpFlCcMueZW65ITe+qFmfZ3opvU
NNyvJz5jAgMBAAECggEBAKXqZJZkultgtiW8v9/b6uNcaD3bUCN08H4oHEIbGWMz
Z6QV876IK1nsU01GgBqJSCCnhObrFHdFnE/laOwmM+OJC7ca/8mU7jq58Su/4jPQ
oOU+VJGXHhOhZN2PD4whb9lvLBoH4HSbYHybZBBBXu6DSRCd2saP1A+4f1ToXPh3
b3+2ioGJJrOrRLkSZjfp6omiX6SjA5jALxK8rfUTQ5HjaFVRyz1XyyfBfipwLjwl
5NTLMBvxdJA3fn1lZjKQPYWvqrTX5zsX20PTBuaTtDBfPPVHvDd7fZJvQ9/JKZW2
DTIQ1UNtt6PNREfKHZYZXJr3VINR+Rn9HL8q3VGcb4ECgYEA4Qhjwf0WXqp1ABdV
QnHURVs/HtbYbEU6bRNZNoaxaKr9PCnCJwkJrHie+FemVH9W7wCUN4zLt2Th6uTw
z2j4FVgY5sclwH0nMQGcTiiq3KIoaTPKR7h0Q5Az1e12PSakkQyAyQVYKaDtfUmY
bG13jE+NxKruSJCUdFzEgsU7HQMCgYEAxufhaSb4DnjP7VTDkgyTjzZ6/SeJIurM
9ZugYAPqg29B2nD4DbovggfBu8WvYotOMjMJNTYB/7Nc5f3ZTO8b9i+mJdx/6pWL
lbRTXhaNKMaUx2nUUFs1rhBzTYSYpb1A3mYbjXe9q0Mtw3xJHJVolY8GvdJHhFOv
yVCQpYnjKyECgYBiRnSyimHTk+Om14nPi8ClTXUidbdsiUs7yYBjlK0zxcD0HlSB
Eaxc2wyp7jVgn4AKvpj8LYvmGrOjDrqwCeqV/7RYTM6K4t1TxJ1LcO01j8fQMeL8
MWzs+LP6kErb591kzy4LHD9lZrdwyMw9Rg04hKGoKvIHVMTQkJbteU8YmwKBgA+D
D/I6ZsgCJf0VSjc+oddeYVMS3UAK3bcdzvEN/SEI8TLO8plndsMGRdaWASqHQK7r
igFLV/aQD2OkW2kDkMOvTZ7QRm2OAhfHu2SwD4wpiHrQxw0JP/N2NvfJqnnqe3+c
qPNsbi9ICu6e57jB8ikPwW/WUVuBh0kE7nLqgPvhAoGARlbPHWAmPksdsvfyLUTM
j/o0MeM1JkGcL7LXPxWztI9v2uZ4K518AaA41g1OT8qqEZqw4cLpTXXvQP7/a4m9
TK4Whre81BaislgY3okTc7rh3BQqwsuSYzX8xwrN+mHLo6UQxQNdsufk2tBsmjHp
E21Tpu8bZrCe/CI4SMNSPuE=
-----END PRIVATE KEY-----
`
			secret_name, secret, err := CertificateToSecret("akkeris.example.com", []byte(cert), []byte(key), "istio-system")
			So(err, ShouldBeNil)
			So(*secret_name, ShouldEqual, "akkeris-example-com-tls")
			So(secret.GetName(), ShouldEqual, "akkeris-example-com-tls")
			So(secret.GetNamespace(), ShouldEqual, "istio-system")
			So(secret.Type, ShouldEqual, kube.SecretTypeTLS)
			So(secret.Kind, ShouldEqual, "Secret")
			So(secret.APIVersion, ShouldEqual, "v1")
			So(string(secret.Data["tls.crt"]), ShouldEqual, "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUR0RENDQXB3Q0NRRCtjYXVIb3RpUURqQU5CZ2txaGtpRzl3MEJBUXNGQURDQm16RUxNQWtHQTFVRUJoTUMKVlZNeERUQUxCZ05WQkFnTUJGVjBZV2d4RnpBVkJnTlZCQWNNRGxOaGJIUWdUR0ZyWlNCRGFYUjVNUkF3RGdZRApWUVFLREFkQmEydGxjbWx6TVJBd0RnWURWUVFMREFkVVpYTjBhVzVuTVJ3d0dnWURWUVFEREJOaGEydGxjbWx6CkxtVjRZVzF3YkdVdVkyOXRNU0l3SUFZSktvWklodmNOQVFrQkZoTmhhMnRsY21selFHVjRZVzF3YkdVdVkyOXQKTUI0WERUSXdNREV4TlRFM05UUTBNRm9YRFRJeE1ERXhOREUzTlRRME1Gb3dnWnN4Q3pBSkJnTlZCQVlUQWxWVApNUTB3Q3dZRFZRUUlEQVJWZEdGb01SY3dGUVlEVlFRSERBNVRZV3gwSUV4aGEyVWdRMmwwZVRFUU1BNEdBMVVFCkNnd0hRV3RyWlhKcGN6RVFNQTRHQTFVRUN3d0hWR1Z6ZEdsdVp6RWNNQm9HQTFVRUF3d1RZV3RyWlhKcGN5NWwKZUdGdGNHeGxMbU52YlRFaU1DQUdDU3FHU0liM0RRRUpBUllUWVd0clpYSnBjMEJsZUdGdGNHeGxMbU52YlRDQwpBU0l3RFFZSktvWklodmNOQVFFQkJRQURnZ0VQQURDQ0FRb0NnZ0VCQUs3WVVkN2NLbUlRMFZhUVQ4L0ZiU3d5Ci9oZFFSU2FYZStSbjhQV2gyT01VME9vUTZOZlFyM2Qvam0wb2lUSTc5bVJiVjJzRXYzQ0cxZTJkYkdSRnR6N1AKNks5cmdnNTF0bGovWmdsWEl0a3RhREc1SDJ5bHZmZXV3cXRYYVoyZkpocnExdmJhYldaZjVvdEVJU1RaREErRgpFTWxPQ1ZxdHpsSTdXdGF4MGd3NXhWZTltVzJmb1RyMGVrL3BraGJKcFlJdHpXcFpaTEJSK1dmaUp1dHVwdzc0CnNzOFkwZlR2L21sZ3ZIWmYxZitXd0Q4UkgrVkg2TFp3aFQ4TDB0TWVySW9kTDJxMkhBbzAzM2xISkRiVkFnck8KNlpXK1dKVDc1L1FqSklkREVEdG15eHExZGpIQUtrV1VKd3k1NWxicmtoTjc2b1daOW5laW05UTAzSzhuUG1NQwpBd0VBQVRBTkJna3Foa2lHOXcwQkFRc0ZBQU9DQVFFQUVMTVZuaVo4YVVCOG9kTkhtU3VEa1pjMVd6SW90TGlECnZNNmFMZXRRS3lSODI2L1dBVGd1VDRBcDNGVE5kMXd6N2FUM0JLUlVJWTZBenFaMlRSLzMvRTNRWlhGZnlXTmUKZ1BLTHRPd25uaU5MU3ZPK0JJTTRzNjB0R3c3bHZzYlhCRjB2V0txTWVEaFF3TzdCK3J1RnlSRTROVDhkU1ZSZQoyWE5tcmtHaUdYeUFXcmxsMEVzVVBOTlMzcWZ1bk1PY1h6UGxyaWVrTWpjNlBlS3hUVXllYzcxTHF5ODgzN3dmCkh0TmZNRmtzYnlsVHNBRjFkQ2Foc2tZeTFJTnlvSGorVU9CcSs3ZGRieGdJV2NDYnljZXBVZnYvZjk3TFM1M04KQmNuZFRhZWdBekROVzBnKzBrd0lWL3VJWGcxK1dQMGx0SkNnc3dFVHFLQXB6RXhOK3c0cWpRPT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=")
			So(string(secret.Data["tls.key"]), ShouldEqual, "LS0tLS1CRUdJTiBQUklWQVRFIEtFWS0tLS0tCk1JSUV2UUlCQURBTkJna3Foa2lHOXcwQkFRRUZBQVNDQktjd2dnU2pBZ0VBQW9JQkFRQ3UyRkhlM0NwaUVORlcKa0UvUHhXMHNNdjRYVUVVbWwzdmtaL0Qxb2RqakZORHFFT2pYMEs5M2Y0NXRLSWt5Ty9aa1cxZHJCTDl3aHRYdApuV3hrUmJjK3oraXZhNElPZGJaWS8yWUpWeUxaTFdneHVSOXNwYjMzcnNLclYybWRueVlhNnRiMjJtMW1YK2FMClJDRWsyUXdQaFJESlRnbGFyYzVTTzFyV3NkSU1PY1ZYdlpsdG42RTY5SHBQNlpJV3lhV0NMYzFxV1dTd1VmbG4KNGlicmJxY08rTExQR05IMDcvNXBZTHgyWDlYL2xzQS9FUi9sUitpMmNJVS9DOUxUSHF5S0hTOXF0aHdLTk45NQpSeVEyMVFJS3p1bVZ2bGlVKytmMEl5U0hReEE3WnNzYXRYWXh3Q3BGbENjTXVlWlc2NUlUZStxRm1mWjNvcHZVCk5OeXZKejVqQWdNQkFBRUNnZ0VCQUtYcVpKWmt1bHRndGlXOHY5L2I2dU5jYUQzYlVDTjA4SDRvSEVJYkdXTXoKWjZRVjg3NklLMW5zVTAxR2dCcUpTQ0NuaE9ickZIZEZuRS9sYU93bU0rT0pDN2NhLzhtVTdqcTU4U3UvNGpQUQpvT1UrVkpHWEhoT2haTjJQRDR3aGI5bHZMQm9INEhTYllIeWJaQkJCWHU2RFNSQ2Qyc2FQMUErNGYxVG9YUGgzCmIzKzJpb0dKSnJPclJMa1NaamZwNm9taVg2U2pBNWpBTHhLOHJmVVRRNUhqYUZWUnl6MVh5eWZCZmlwd0xqd2wKNU5UTE1CdnhkSkEzZm4xbFpqS1FQWVd2cXJUWDV6c1gyMFBUQnVhVHREQmZQUFZIdkRkN2ZaSnZROS9KS1pXMgpEVElRMVVOdHQ2UE5SRWZLSFpZWlhKcjNWSU5SK1JuOUhMOHEzVkdjYjRFQ2dZRUE0UWhqd2YwV1hxcDFBQmRWClFuSFVSVnMvSHRiWWJFVTZiUk5aTm9heGFLcjlQQ25DSndrSnJIaWUrRmVtVkg5Vzd3Q1VONHpMdDJUaDZ1VHcKejJqNEZWZ1k1c2Nsd0gwbk1RR2NUaWlxM0tJb2FUUEtSN2gwUTVBejFlMTJQU2Fra1F5QXlRVllLYUR0ZlVtWQpiRzEzakUrTnhLcnVTSkNVZEZ6RWdzVTdIUU1DZ1lFQXh1ZmhhU2I0RG5qUDdWVERrZ3lUanpaNi9TZUpJdXJNCjladWdZQVBxZzI5QjJuRDREYm92Z2dmQnU4V3ZZb3RPTWpNSk5UWUIvN05jNWYzWlRPOGI5aSttSmR4LzZwV0wKbGJSVFhoYU5LTWFVeDJuVVVGczFyaEJ6VFlTWXBiMUEzbVlialhlOXEwTXR3M3hKSEpWb2xZOEd2ZEpIaEZPdgp5VkNRcFluakt5RUNnWUJpUm5TeWltSFRrK09tMTRuUGk4Q2xUWFVpZGJkc2lVczd5WUJqbEswenhjRDBIbFNCCkVheGMyd3lwN2pWZ240QUt2cGo4TFl2bUdyT2pEcnF3Q2VxVi83UllUTTZLNHQxVHhKMUxjTzAxajhmUU1lTDgKTVd6cytMUDZrRXJiNTkxa3p5NExIRDlsWnJkd3lNdzlSZzA0aEtHb0t2SUhWTVRRa0pidGVVOFltd0tCZ0ErRApEL0k2WnNnQ0pmMFZTamMrb2RkZVlWTVMzVUFLM2JjZHp2RU4vU0VJOFRMTzhwbG5kc01HUmRhV0FTcUhRSzdyCmlnRkxWL2FRRDJPa1cya0RrTU92VFo3UVJtMk9BaGZIdTJTd0Q0d3BpSHJReHcwSlAvTjJOdmZKcW5ucWUzK2MKcVBOc2JpOUlDdTZlNTdqQjhpa1B3Vy9XVVZ1Qmgwa0U3bkxxZ1B2aEFvR0FSbGJQSFdBbVBrc2RzdmZ5TFVUTQpqL28wTWVNMUprR2NMN0xYUHhXenRJOXYydVo0SzUxOEFhQTQxZzFPVDhxcUVacXc0Y0xwVFhYdlFQNy9hNG05ClRLNFdocmU4MUJhaXNsZ1kzb2tUYzdyaDNCUXF3c3VTWXpYOHh3ck4rbUhMbzZVUXhRTmRzdWZrMnRCc21qSHAKRTIxVHB1OGJackNlL0NJNFNNTlNQdUU9Ci0tLS0tRU5EIFBSSVZBVEUgS0VZLS0tLS0K")
		})
	})
}
