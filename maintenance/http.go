package maintenance

import (
	"github.com/go-martini/martini"
)

func AddToMartini(m *martini.ClassicMartini) {
	m.Post("/v1/space/:space/app/:app/maintenance", EnableMaintenancePage)
	m.Delete("/v1/space/:space/app/:app/maintenance", DisableMaintenancePage)
	m.Get("/v1/space/:space/app/:app/maintenance", MaintenancePageStatus)
}