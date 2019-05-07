package service

import (
	"database/sql"
	"errors"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	osb "github.com/pmorie/go-open-service-broker-client/v2"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	utils "region-api/utils"
	"time"
)

type providerInfo struct {
	serviceUrl string
	client     osb.Client
	services   []osb.Service
}

type OSBClientServices struct {
	providersCache []providerInfo
	serviceUrls    []string
	db             *sql.DB
}

type BindResource struct {
	AppGUID *string `json:"appGuid,omitempty"`
}

type BindRequestBody struct {
	ServiceID    string                 `json:"service_id"`
	PlanID       string                 `json:"plan_id"`
	BindResource *BindResource          `json:"bind_resource,omitempty"`
	Parameters   map[string]interface{} `json:"parameters,omitempty"`
	Context      map[string]interface{} `json:"context,omitempty"`
}

type ProvisionRequestBody struct {
	ServiceID        string                 `json:"service_id"`
	PlanID           string                 `json:"plan_id"`
	OrganizationGUID string                 `json:"organization_guid,omitempty"`
	SpaceGUID        string                 `json:"space_guid,omitempty"`
	Parameters       map[string]interface{} `json:"parameters,omitempty"`
	Context          map[string]interface{} `json:"context,omitempty"`
}

type PreviousValues struct {
	PlanID    string `json:"plan_id,omitempty"`
	ServiceID string `json:"service_id,omitempty"`
	OrgID     string `json:"organization_id,omitempty"`
	SpaceID   string `json:"space_id,omitempty"`
}

type UpdateRequestBody struct {
	ServiceID      string                 `json:"service_id"`
	PlanID         *string                `json:"plan_id,omitempty"`
	Parameters     map[string]interface{} `json:"parameters,omitempty"`
	Context        map[string]interface{} `json:"context,omitempty"`
	PreviousValues *PreviousValues        `json:"previous_values,omitempty"`
}

type updateInstanceResponseBody struct {
	DashboardURL *string           `json:"dashboard_url"`
	Operation    *osb.OperationKey `json:"operation"`
}

type provisionSuccessResponseBody struct {
	DashboardURL *string           `json:"dashboard_url"`
	Operation    *osb.OperationKey `json:"operation"`
}

type osbErrors struct {
	ErrorMessage *string `json:"error"`
	Description  *string `json:"description"`
}

func ProcessErrors(err error, r render.Render) {
	if herr, is_http := osb.IsHTTPError(err); is_http {
		r.JSON(herr.StatusCode, osbErrors{ErrorMessage: herr.ErrorMessage, Description: herr.Description})
	} else {
		utils.ReportError(err, r)
	}
}

func NewOSBClientServices(serviceUrls []string, db *sql.DB) (*OSBClientServices, error) {
	var osbClientServices OSBClientServices = OSBClientServices{serviceUrls: serviceUrls, db: db}
	osbClientServices.init()
	osbClientServices.GetProviders()
	return &osbClientServices, nil
}

func (cserv *OSBClientServices) init() {
	go (func() {
		t := time.NewTicker(time.Minute * 30)
		for {
			cserv.providersCache = nil
			<-t.C
		}
	})()
}

func (cserv *OSBClientServices) GetProviders() ([]providerInfo, error) {
	if cserv.providersCache != nil {
		return cserv.providersCache, nil
	}
	services := make([]providerInfo, 0)
	for _, serviceUrl := range cserv.serviceUrls {
		config := osb.DefaultClientConfiguration()
		sUrl, err := url.Parse(serviceUrl)
		if err == nil {
			config.URL = serviceUrl
			if sUrl.User != nil && sUrl.User.Username() != "" {
				pwd, _ := sUrl.User.Password()
				config.AuthConfig = &osb.AuthConfig{BasicAuthConfig: &osb.BasicAuthConfig{Username: sUrl.User.Username(), Password: pwd}}
			} else if sUrl.User != nil && sUrl.User.Username() == "" {
				pwd, _ := sUrl.User.Password()
				config.AuthConfig = &osb.AuthConfig{BearerConfig: &osb.BearerConfig{Token: pwd}}
			}
			config.EnableAlphaFeatures = true // necessary for GetBinding
			client, err := osb.NewClient(config)
			if err != nil {
				return nil, err
			}
			s, err := client.GetCatalog()
			if err != nil {
				return nil, err
			}
			service := providerInfo{serviceUrl: serviceUrl, client: client, services: s.Services}
			services = append(services, service)
		} else {
			log.Printf("ERROR: Unable to parse service url %s\n", serviceUrl)
		}
	}
	cserv.providersCache = services
	return cserv.providersCache, nil
}

func (cserv *OSBClientServices) GetOSBServices() (*osb.CatalogResponse, error) {
	if cserv.providersCache == nil {
		cserv.GetProviders()
	}
	var catalog osb.CatalogResponse
	for _, provider := range cserv.providersCache {
		catalog.Services = append(catalog.Services, provider.services...)
	}
	return &catalog, nil
}

func (cserv *OSBClientServices) GetPlansByID(serviceId string) ([]osb.Plan, error) {
	service, err := cserv.GetServiceByID(serviceId)
	if err != nil {
		return nil, err
	}
	return service.Plans, nil
}

func (cserv *OSBClientServices) GetServiceByID(serviceId string) (*osb.Service, error) {
	providers, err := cserv.GetProviders()
	if err != nil {
		return nil, err
	}
	for _, p := range providers {
		for _, s := range p.services {
			if s.ID == serviceId {
				return &s, nil
			}
		}
	}
	return nil, errors.New("Unable to find service")
}

func (cserv *OSBClientServices) GetServiceByName(serviceName string) (*osb.Service, error) {
	providers, err := cserv.GetProviders()
	if err != nil {
		return nil, err
	}
	for _, p := range providers {
		for _, s := range p.services {
			if s.Name == serviceName {
				return &s, nil
			}
		}
	}
	return nil, errors.New("Unable to find service")
}

func (cserv *OSBClientServices) GetPlanByID(serviceId string, planId string) (*osb.Plan, error) {
	service, err := cserv.GetServiceByID(serviceId)
	if err != nil {
		return nil, err
	}
	for _, p := range service.Plans {
		if p.ID == planId {
			return &p, nil
		}
	}
	return nil, errors.New("Unable to find plan")
}

func (cserv *OSBClientServices) GetPlansByName(serviceName string) ([]osb.Plan, error) {
	service, err := cserv.GetServiceByName(serviceName)
	if err != nil {
		return nil, err
	}
	return service.Plans, nil
}

func (cserv *OSBClientServices) GetProviderByID(serviceId string) (*providerInfo, error) {
	providers, err := cserv.GetProviders()
	if err != nil {
		return nil, err
	}
	var provider *providerInfo = nil
	for _, s := range providers {
		for _, si := range s.services {
			if si.ID == serviceId {
				provider = &s
			}
		}
	}
	if provider == nil {
		return nil, errors.New("Unable to find service")
	}
	return provider, nil
}

func (cserv *OSBClientServices) GetProviderByName(serviceName string) (*providerInfo, error) {
	providers, err := cserv.GetProviders()
	if err != nil {
		return nil, err
	}
	var provider *providerInfo = nil
	for _, s := range providers {
		for _, si := range s.services {
			if si.Name == serviceName {
				provider = &s
			}
		}
	}
	if provider == nil {
		return nil, errors.New("Unable to find service")
	}
	return provider, nil
}

func (cserv *OSBClientServices) GetInstanceInfoByID(instanceId string) (serviceId string, planId string, operationKey *string, status string, err error) {
	err = cserv.db.QueryRow("select instance_id, service_id, plan_id, operation_key, status from service_instances where instance_id = $1", instanceId).Scan(&instanceId, &serviceId, &planId, &operationKey, &status)
	return serviceId, planId, operationKey, status, err
}

func (cserv *OSBClientServices) InsertInstanceInfo(instanceId string, serviceId string, planId string, operationKey *string, status osb.LastOperationState) (err error) {
	_, err = cserv.db.Exec("insert into service_instances (instance_id, service_id, plan_id, operation_key, status) values ($1, $2, $3, $4, $5)", instanceId, serviceId, planId, operationKey, status)
	return err
}

func (cserv *OSBClientServices) UpdateInstanceInfo(instanceId string, serviceId string, planId string, operationKey *string, status osb.LastOperationState) (err error) {
	_, err = cserv.db.Exec("update service_instances set service_id = $2, plan_id = $3, operation_key = $4, status = $5 where instance_id = $1", instanceId, serviceId, planId, operationKey, status)
	return err
}

func (cserv *OSBClientServices) RemoveInstanceInfo(instanceId string) (err error) {
	_, err = cserv.db.Exec("delete from service_instances where instance_id = $1", instanceId)
	return err
}

func (cserv *OSBClientServices) Provision(instanceId string, service *osb.Service, plan *osb.Plan, orgGuid string, spaceGuid string) (*osb.ProvisionResponse, error) {
	request := &osb.ProvisionRequest{
		InstanceID:        instanceId,
		ServiceID:         service.ID,
		PlanID:            plan.ID,
		OrganizationGUID:  orgGuid,
		SpaceGUID:         spaceGuid,
		Parameters:        map[string]interface{}{},
		AcceptsIncomplete: true,
	}

	svc, err := cserv.GetProviderByID(service.ID)
	if err != nil {
		return nil, err
	}

	resp, err := svc.client.ProvisionInstance(request)
	if err != nil {
		return nil, err
	}
	if resp.Async {
		var opkey *string = nil
		if resp.OperationKey != nil {
			tmpkey := string(*resp.OperationKey)
			opkey = &tmpkey
		}
		if err = cserv.InsertInstanceInfo(instanceId, service.ID, plan.ID, opkey, osb.StateInProgress); err != nil {
			log.Printf("ERROR: Cannot record instance %s %s %s because %s\n", instanceId, service.ID, plan.ID, err.Error())
		}
	} else {
		if err = cserv.InsertInstanceInfo(instanceId, service.ID, plan.ID, nil, osb.StateInProgress); err != nil {
			log.Printf("ERROR: Cannot record instance %s %s %s because %s\n", instanceId, service.ID, plan.ID, err.Error())
		}
	}

	return resp, nil
}

func (cserv *OSBClientServices) Update(instanceId string, service *osb.Service, plan *osb.Plan) (*osb.UpdateInstanceResponse, error) {
	request := &osb.UpdateInstanceRequest{
		InstanceID:        instanceId,
		ServiceID:         service.ID,
		PlanID:            &plan.ID,
		Parameters:        map[string]interface{}{},
		AcceptsIncomplete: true,
	}

	svc, err := cserv.GetProviderByID(service.ID)
	if err != nil {
		return nil, err
	}

	resp, err := svc.client.UpdateInstance(request)
	if err != nil {
		return nil, err
	}
	if resp.Async {
		var opkey *string = nil
		if resp.OperationKey != nil {
			tmpkey := string(*resp.OperationKey)
			opkey = &tmpkey
		}
		cserv.UpdateInstanceInfo(instanceId, service.ID, plan.ID, opkey, osb.StateInProgress)
	} else {
		cserv.UpdateInstanceInfo(instanceId, service.ID, plan.ID, nil, osb.StateSucceeded)
	}
	return resp, nil
}

func (cserv *OSBClientServices) Deprovision(instanceId string, service *osb.Service, plan *osb.Plan) (*osb.DeprovisionResponse, error) {
	request := &osb.DeprovisionRequest{
		InstanceID:        instanceId,
		ServiceID:         service.ID,
		PlanID:            plan.ID,
		AcceptsIncomplete: true,
	}

	svc, err := cserv.GetProviderByID(service.ID)
	if err != nil {
		return nil, err
	}

	resp, err := svc.client.DeprovisionInstance(request)
	if err != nil {
		return nil, err
	}
	if err = cserv.RemoveInstanceInfo(instanceId); err != nil {
		log.Printf("ERROR: Cannot record instance %s because %s\n", instanceId, err.Error())
	}
	return resp, nil
}

func (cserv *OSBClientServices) CreateBinding(bindingId string, instanceId string, serviceId string, planId string, appGuid *string) (*osb.BindResponse, error) {
	resource := &osb.BindResource{
		AppGUID: appGuid,
	}
	request := &osb.BindRequest{
		BindingID:    bindingId,
		InstanceID:   instanceId,
		ServiceID:    serviceId,
		PlanID:       planId,
		BindResource: resource,
		Parameters:   map[string]interface{}{},
	}

	svc, err := cserv.GetProviderByID(serviceId)
	if err != nil {
		return nil, err
	}

	response, err := svc.client.Bind(request)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (cserv *OSBClientServices) GetBinding(bindingId string, instanceId string, serviceId string) (*osb.GetBindingResponse, error) {
	request := &osb.GetBindingRequest{
		BindingID:  bindingId,
		InstanceID: instanceId,
	}

	svc, err := cserv.GetProviderByID(serviceId)
	if err != nil {
		return nil, err
	}

	return svc.client.GetBinding(request)
}

func (cserv *OSBClientServices) RemoveBinding(bindingId string, instanceId string, serviceId string, planId string) (*osb.UnbindResponse, error) {
	request := &osb.UnbindRequest{
		BindingID:  bindingId,
		InstanceID: instanceId,
		ServiceID:  serviceId,
		PlanID:     planId,
	}

	svc, err := cserv.GetProviderByID(serviceId)
	if err != nil {
		return nil, err
	}

	return svc.client.Unbind(request)
}

func (cserv *OSBClientServices) GetInstanceStatus(instanceId string) (*osb.LastOperationResponse, error) {
	serviceId, planId, operationKey, _, err := cserv.GetInstanceInfoByID(instanceId)
	if err != nil {
		return nil, err
	}

	svc, err := cserv.GetProviderByID(serviceId)
	if err != nil {
		return nil, err
	}

	var opkey *osb.OperationKey = nil
	if operationKey != nil {
		tmpopkey := osb.OperationKey(*operationKey)
		opkey = &tmpopkey
	}
	request := &osb.LastOperationRequest{
		InstanceID:   instanceId,
		ServiceID:    &serviceId,
		PlanID:       &planId,
		OperationKey: opkey,
	}

	response, err := svc.client.PollLastOperation(request)
	if err != nil {
		return nil, err
	}

	if err = cserv.UpdateInstanceInfo(instanceId, serviceId, planId, operationKey, response.State); err != nil {
		return nil, err
	}

	return response, nil
}

func (cserv *OSBClientServices) HttpGetCatalog(params martini.Params, r render.Render) {
	catalog, err := cserv.GetOSBServices()
	if err != nil {
		ProcessErrors(err, r)
		return
	}
	r.JSON(http.StatusOK, catalog)
}

func (cserv *OSBClientServices) HttpGetLastOperation(params martini.Params, r render.Render) {
	resp, err := cserv.GetInstanceStatus(params["instance_id"])
	if err != nil {
		ProcessErrors(err, r)
		return
	}
	r.JSON(http.StatusOK, resp)
}

func (cserv *OSBClientServices) HttpPartialUpdateInstance(params martini.Params, spec UpdateRequestBody, berr binding.Errors, r render.Render) {
	if berr != nil {
		log.Println("Failed to unserialize request")
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}

	instanceId := params["instance_id"]
	service, err := cserv.GetServiceByID(spec.ServiceID)
	if err != nil {
		ProcessErrors(err, r)
		return
	}
	if spec.PlanID == nil {
		r.JSON(http.StatusNotFound, map[string]interface{}{"error": "PlanNotFound", "description": "The specified plan was not found."})
		return
	}
	plan, err := cserv.GetPlanByID(spec.ServiceID, *spec.PlanID)
	if err != nil && err.Error() == "Unable to find service" {
		r.JSON(http.StatusNotFound, map[string]interface{}{"error": "ServiceNotFound", "description": "The specified service was not found."})
		return
	} else if err != nil && err.Error() == "Unable to find plan" {
		r.JSON(http.StatusNotFound, map[string]interface{}{"error": "PlanNotFound", "description": "The specified plan was not found."})
		return
	} else if err != nil {
		ProcessErrors(err, r)
		return
	}

	_, _, _, status, err := cserv.GetInstanceInfoByID(instanceId)

	if err != nil {
		ProcessErrors(err, r)
		return
	} else if status != string(osb.StateInProgress) {
		resp, err := cserv.Update(instanceId, service, plan)
		if err != nil {
			ProcessErrors(err, r)
			return
		}
		r.JSON(http.StatusOK, updateInstanceResponseBody{DashboardURL: nil, Operation: resp.OperationKey})
	} else {
		r.JSON(http.StatusConflict, map[string]interface{}{"error": "ProvisionInProgress", "description": "This instance cannot be provisioned or updated because a provision is already in progress."})
	}
}

func (cserv *OSBClientServices) HttpGetCreateOrUpdateInstance(params martini.Params, spec ProvisionRequestBody, berr binding.Errors, r render.Render) {
	if berr != nil {
		log.Println("Failed to unserialize request")
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}

	instanceId := params["instance_id"]

	if spec.OrganizationGUID == "" {
		spec.OrganizationGUID = "00000000-0000-0000-0000-000000000000"
	}
	if spec.SpaceGUID == "" {
		spec.SpaceGUID = "00000000-0000-0000-0000-000000000000"
	}

	service, err := cserv.GetServiceByID(spec.ServiceID)
	if err != nil {
		ProcessErrors(err, r)
		return
	}

	plan, err := cserv.GetPlanByID(spec.ServiceID, spec.PlanID)
	if err != nil && err.Error() == "Unable to find service" {
		r.JSON(http.StatusNotFound, map[string]interface{}{"error": "ServiceNotFound", "description": "The specified service was not found."})
		return
	} else if err != nil && err.Error() == "Unable to find plan" {
		r.JSON(http.StatusNotFound, map[string]interface{}{"error": "PlanNotFound", "description": "The specified plan was not found."})
		return
	} else if err != nil {
		ProcessErrors(err, r)
		return
	}

	_, _, _, status, err := cserv.GetInstanceInfoByID(instanceId)

	if err != nil {
		// The instance does not exist
		resp, err := cserv.Provision(instanceId, service, plan, spec.OrganizationGUID, spec.SpaceGUID)
		if err != nil {
			ProcessErrors(err, r)
			return
		}

		if resp.Async {
			r.JSON(http.StatusOK, provisionSuccessResponseBody{DashboardURL: nil, Operation: resp.OperationKey})
		} else {
			r.JSON(http.StatusCreated, provisionSuccessResponseBody{DashboardURL: nil, Operation: resp.OperationKey})
		}
	} else if status != string(osb.StateInProgress) {
		resp, err := cserv.Update(instanceId, service, plan)
		if err != nil {
			ProcessErrors(err, r)
			return
		}
		r.JSON(http.StatusOK, provisionSuccessResponseBody{DashboardURL: nil, Operation: resp.OperationKey})
	} else {
		r.JSON(http.StatusConflict, map[string]interface{}{"error": "ProvisionInProgress", "description": "This instance cannot be provisioned or updated because a provision is already in progress."})
	}
}

func (cserv *OSBClientServices) HttpDeleteInstance(params martini.Params, r render.Render) {
	serviceId, planId, _, _, err := cserv.GetInstanceInfoByID(params["instance_id"])
	if err != nil {
		ProcessErrors(err, r)
		return
	}

	service, err := cserv.GetServiceByID(serviceId)
	if err != nil {
		ProcessErrors(err, r)
		return
	}

	plan, err := cserv.GetPlanByID(serviceId, planId)
	if err != nil {
		ProcessErrors(err, r)
		return
	}

	resp, err := cserv.Deprovision(params["instance_id"], service, plan)
	if err != nil {
		ProcessErrors(err, r)
		return
	}

	r.JSON(http.StatusOK, resp)
}

func (cserv *OSBClientServices) HttpCreateOrUpdateBinding(params martini.Params, spec BindRequestBody, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	var appGuid *string = nil
	if spec.BindResource != nil {
		appGuid = spec.BindResource.AppGUID
	}
	resp, err := cserv.CreateBinding(params["binding_id"], params["instance_id"], spec.ServiceID, spec.PlanID, appGuid)
	if err != nil {
		ProcessErrors(err, r)
		return
	}

	r.JSON(http.StatusOK, resp)
}

func (cserv *OSBClientServices) HttpGetBinding(params martini.Params, r render.Render) {
	serviceId, _, _, _, err := cserv.GetInstanceInfoByID(params["instance_id"])
	if err != nil && err.Error() == "sql: no rows in result set" {
		r.JSON(http.StatusNotFound, map[string]interface{}{"error": "NotFound", "description": "The instance could not be found."})
		return
	}
	if err != nil {
		ProcessErrors(err, r)
		return
	}

	resp, err := cserv.GetBinding(params["binding_id"], params["instance_id"], serviceId)
	if err != nil {
		ProcessErrors(err, r)
		return
	}

	r.JSON(http.StatusOK, resp)
}

func (cserv *OSBClientServices) HttpGetBindingLastOperation(params martini.Params, r render.Render) {
	r.JSON(http.StatusOK,
		map[string]interface{}{"status": "succeeded", "description": ""}) // TOOD ? Does this interface really exist
}

func (cserv *OSBClientServices) HttpRemoveBinding(params martini.Params, r render.Render) {
	serviceId, planId, _, _, err := cserv.GetInstanceInfoByID(params["instance_id"])
	if err != nil {
		ProcessErrors(err, r)
		return
	}

	resp, err := cserv.RemoveBinding(params["binding_id"], params["instance_id"], serviceId, planId)
	if err != nil {
		ProcessErrors(err, r)
		return
	}

	r.JSON(http.StatusOK, resp)
}

func (cserv *OSBClientServices) HttpForwardAction(params martini.Params, res http.ResponseWriter, req *http.Request) {
	serviceId, _, _, _, err := cserv.GetInstanceInfoByID(params["instance_id"])
	if err != nil {
		log.Println("Error: " + err.Error())
		res.WriteHeader(500)
		res.Write([]byte("Internal Server Error"))
		return
	}

	provider, err := cserv.GetProviderByID(serviceId)
	if err != nil {
		log.Println("Error: " + err.Error())
		res.WriteHeader(500)
		res.Write([]byte("Internal Server Error"))
		return
	}
	uri, err := url.Parse(provider.serviceUrl)
	if err != nil {
		log.Println("Error: " + err.Error())
		res.WriteHeader(500)
		res.Write([]byte("Internal Server Error"))
		return
	}
	rp := httputil.NewSingleHostReverseProxy(uri)
	req.Host = uri.Host
	rp.FlushInterval = time.Duration(200) * time.Millisecond
	rp.ServeHTTP(res, req)
}

var globalClientService *OSBClientServices = nil

func SetGlobalClientService(cserv *OSBClientServices) {
	globalClientService = cserv
}

// The servicetype can either be an id or service name.
func GetOSBBindingCredentials(servicetype string, instanceId string, bindingId string) (map[string]interface{}, error) {
	if globalClientService == nil {
		return nil, errors.New("Open service brokers are not configured.")
	}
	provider, err := globalClientService.GetProviderByID(servicetype)
	if err != nil {
		provider, err = globalClientService.GetProviderByName(servicetype)
		if err != nil {
			return nil, errors.New("No such servicetype")
		}
	}

	request := &osb.GetBindingRequest{
		BindingID:  bindingId,
		InstanceID: instanceId,
	}

	resp, err := provider.client.GetBinding(request)
	if err != nil {
		log.Printf("ERROR: Unable to get bind credentials for service: %s because: %s\n", servicetype, err.Error())
		return nil, err
	}

	return resp.Credentials, nil
}

func IsOSBService(servicetype string) bool {
	if globalClientService == nil {
		return false
	}
	if _, err := globalClientService.GetProviderByID(servicetype); err != nil {
		if _, err = globalClientService.GetProviderByName(servicetype); err != nil {
			return false
		}
	}
	return true
}
