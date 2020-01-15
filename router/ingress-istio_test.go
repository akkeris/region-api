package router

import (
	. "github.com/smartystreets/goconvey/convey"
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

	Convey("Gateways should be maniputable", t, func(c C) {

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
	})

}
