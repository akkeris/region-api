package templates

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
	"os"
	structs "region-api/structs"
	utils "region-api/utils"
)

func GetURLTemplates(db *sql.DB, params martini.Params, r render.Render) {
	urltemplates, err := getURLTemplates()
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(200, urltemplates)
}

func getURLTemplates() (u structs.URLTemplates, e error) {
	var urltemplates structs.URLTemplates
	urltemplates.Internal = os.Getenv("ALAMO_INTERNAL_URL_TEMPLATE")
	urltemplates.External = os.Getenv("ALAMO_URL_TEMPLATE")
	if urltemplates.Internal == "" {
		fmt.Println("No internal url template")
		return urltemplates, errors.New("No Inernal URL Template")
	}
	if urltemplates.External == "" {
		fmt.Println("No external url template")
		return urltemplates, errors.New("No External URL Template")
	}
	return urltemplates, nil
}

const Snirule = `
{{ $domain := .Domain }}
when HTTP_REQUEST {
	switch [string tolower [HTTP::host]] {
		"{{.Domain}}" {
			set xri [HTTP::header "x-request-id"]
			set hrt [clock format [clock seconds] -gmt 0 -format "%m-%d-%YT%H:%M:%S%z"]
			set http_request_start_time [clock clicks -milliseconds]
			if {$xri eq ""} {
			    binary scan [md5 "[IP::client_addr][TCP::client_port][IP::local_addr][TCP::local_port][string range [AES::key 256] 8 end]"] H* xri junk
			}
			set LogString "timestamp=$hrt fwd=[IP::client_addr] method=[HTTP::method] path=[HTTP::uri] request_id=$xri site_domain={{$domain}} "
		    HTTP::header insert x-request-id $xri	
			HTTP::header insert X-Orig-Proto [HTTP::header "X-Forwarded-Proto"]
			HTTP::header insert X-Orig-Host [HTTP::header "Host"]
			HTTP::header insert X-Orig-Port [TCP::local_port]
			HTTP::header insert X-Forwarded-Path [HTTP::path]
			switch -glob [string tolower [HTTP::uri]] {
{{ range $value := .Switches }}
"{{$value.Path}}/*" {
set LogString "$LogString hostname={{$value.NewHost}} site_path=[HTTP::path]"
HTTP::header insert X-Orig-Path "{{$value.Path}}"
HTTP::path [string map -nocase {"{{$value.Path}}/" "{{$value.ReplacePath}}/"} [HTTP::path]]
if {[regsub -all "//" [HTTP::path] "/" newpath] > 0} { HTTP::path $newpath }
pool {{$value.Pool}}
}
"{{$value.Path}}*" {
set LogString "$LogString hostname={{$value.NewHost}} site_path=[HTTP::uri]"
HTTP::header insert X-Orig-Path "{{$value.Path}}"
HTTP::uri [string map -nocase {"{{$value.Path}}" "{{$value.ReplacePath}}"} [HTTP::uri]]
pool {{$value.Pool}}
}
{{end}}
			}
		}
	}
}
`
