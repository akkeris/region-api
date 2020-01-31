package vault

import (
	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
	"net/http"
)

func HttpGetVaultList(params martini.Params, r render.Render) {
	r.JSON(http.StatusOK, GetList())
}

func HttpGetVaultVariablesMasked(params martini.Params, r render.Render) {
	r.JSON(http.StatusOK, GetVaultVariablesMasked(params["_1"]))
}

func AddToMartini(m *martini.ClassicMartini) {
	m.Get("/v1/octhc/service/vault", HttpGetVaultList)
	m.Get("/v1/service/vault/plans", HttpGetVaultList)
	m.Get("/v1/service/vault/credentials/**", HttpGetVaultVariablesMasked)
}