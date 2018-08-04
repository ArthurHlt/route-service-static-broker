package main

import (
	"code.cloudfoundry.org/lager"
	"context"
	"flag"
	"fmt"
	"github.com/cloudfoundry-community/gautocloud"
	"github.com/cloudfoundry-community/gautocloud/connectors/generic"
	"github.com/cloudfoundry-community/gautocloud/logger"
	"github.com/pivotal-cf/brokerapi"
	"github.com/satori/go.uuid"
	"log"
	"net/http"
	"os"
)

const (
	ROOT_UUID = "aaa4b55e-5768-41ea-a383-5f633725a88a"
)

func init() {
	gautocloud.RegisterConnector(generic.NewConfigGenericConnector(RouteSvcStaticConfig{}))
}

type RouteSvcStaticConfig struct {
	RouteServices  []RouteSvc `cloud:"route_services"`
	BrokerUsername string     `cloud:"broker_username" cloud-default:"brokeruser"`
	BrokerPassword string     `cloud:"broker_password" cloud-default:"password"`
}
type RouteSvcStaticBroker struct {
	routeServices []RouteSvc
}

func NewRouteSvcStaticBroker(routeServices []RouteSvc) *RouteSvcStaticBroker {
	return &RouteSvcStaticBroker{routeServices}
}

type RouteSvc struct {
	Name        string
	Id          string `cloud:"-"`
	Description string
	Url         string
	Tags        []string
	Plans       []Plan
}

func (r *RouteSvc) prepare() (RouteSvc, error) {
	if r.Name == "" {
		return RouteSvc{}, fmt.Errorf("Route must have a name")
	}
	if r.Description == "" {
		r.Description = fmt.Sprintf("Route service %s", r.Name)
	}
	if r.Tags == nil || len(r.Tags) == 0 {
		r.Tags = []string{"route-service"}
	}
	if r.Plans == nil || len(r.Plans) == 0 {
		r.Plans = []Plan{
			{
				Name:        "plan-" + r.Name,
				Description: fmt.Sprintf("Default plan for route service %s forwarding to url %s", r.Name, r.Url),
				Url:         r.Url,
			},
		}
	}
	for i, plan := range r.Plans {
		finalPlan, err := plan.prepare()
		if err != nil {
			return RouteSvc{}, err
		}
		r.Plans[i] = finalPlan
	}
	r.Id = uuid.NewV3(uuid.FromStringOrNil(ROOT_UUID), r.Name).String()

	return *r, nil
}

type Plan struct {
	Name        string
	Description string
	Url         string
	Id          string `cloud:"-"`
}

func (p *Plan) prepare() (Plan, error) {
	if p.Url == "" {
		return Plan{}, fmt.Errorf("Plan '%s' must have an url", p.Name)
	}
	p.Id = uuid.NewV3(uuid.FromStringOrNil(ROOT_UUID), p.Name).String()
	return *p, nil
}

func (b *RouteSvcStaticBroker) findRouteUrl(serviceId, planId string) (string, error) {
	var service RouteSvc
	for _, svc := range b.routeServices {
		if svc.Id == serviceId {
			service = svc
			break
		}
	}
	if service.Id == "" {
		return "", fmt.Errorf("Service with id %s can't be found", serviceId)
	}
	var plan Plan
	for _, planTmp := range service.Plans {
		if planTmp.Id == planId {
			plan = planTmp
			break
		}
	}
	if plan.Id == "" {
		return "", fmt.Errorf("Plan with id %s can't be found in service %s ", planId, serviceId)
	}
	return plan.Url, nil
}
func (b *RouteSvcStaticBroker) Services(context.Context) []brokerapi.Service {
	services := make([]brokerapi.Service, 0)
	for _, routeSvc := range b.routeServices {
		plans := make([]brokerapi.ServicePlan, 0)
		for _, plan := range routeSvc.Plans {
			plans = append(plans, brokerapi.ServicePlan{
				ID:          plan.Id,
				Name:        plan.Name,
				Description: plan.Description,
			})
		}
		services = append(services, brokerapi.Service{
			ID:            routeSvc.Id,
			Name:          routeSvc.Name,
			Description:   routeSvc.Description,
			Bindable:      true,
			Tags:          routeSvc.Tags,
			PlanUpdatable: false,
			Requires:      []brokerapi.RequiredPermission{brokerapi.PermissionRouteForwarding},
			Plans:         plans,
		})
	}
	return services
}

func (b *RouteSvcStaticBroker) Provision(context context.Context, instanceID string, details brokerapi.ProvisionDetails, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, error) {
	return brokerapi.ProvisionedServiceSpec{}, nil
}

func (b *RouteSvcStaticBroker) Deprovision(context context.Context, instanceID string, details brokerapi.DeprovisionDetails, asyncAllowed bool) (brokerapi.DeprovisionServiceSpec, error) {
	return brokerapi.DeprovisionServiceSpec{}, nil
}

func (b *RouteSvcStaticBroker) Bind(context context.Context, instanceID string, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, error) {
	url, err := b.findRouteUrl(details.ServiceID, details.PlanID)
	if err != nil {
		return brokerapi.Binding{}, brokerapi.NewFailureResponseBuilder(
			err, http.StatusInternalServerError, "internal-server-error",
		).WithEmptyResponse().Build()
	}
	return brokerapi.Binding{
		Credentials:     "",
		RouteServiceURL: url,
	}, nil
}

func (b *RouteSvcStaticBroker) Unbind(context context.Context, instanceID string, bindingID string, details brokerapi.UnbindDetails) error {
	return nil
}

func (b *RouteSvcStaticBroker) LastOperation(context context.Context, instanceID, operationData string) (brokerapi.LastOperation, error) {
	return brokerapi.LastOperation{}, nil
}

func (b *RouteSvcStaticBroker) Update(context context.Context, instanceID string, details brokerapi.UpdateDetails, asyncAllowed bool) (brokerapi.UpdateServiceSpec, error) {
	return brokerapi.UpdateServiceSpec{}, nil
}

func main() {
	debugInit := flag.Bool("debug-init", false, "enable init delog logs")
	flag.Parse()

	conf := &RouteSvcStaticConfig{}
	if *debugInit {
		gautocloud.SetLogger(log.New(os.Stdout, "", log.Ldate|log.Ltime), logger.Ldebug)
	}

	err := gautocloud.Inject(conf)
	if err != nil {
		panic(err)
	}
	if conf.RouteServices == nil || len(conf.RouteServices) == 0 {
		panic(fmt.Errorf("You must have configured route service in your cloud configuration."))
	}
	for i, routeSvc := range conf.RouteServices {
		finalRouteSvc, err := routeSvc.prepare()
		if err != nil {
			panic(fmt.Errorf("Error on route number %d: %s", i, err.Error()))
		}
		conf.RouteServices[i] = finalRouteSvc
	}
	serviceBroker := NewRouteSvcStaticBroker(conf.RouteServices)
	logger := lager.NewLogger("guard-broker")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.ERROR))
	credentials := brokerapi.BrokerCredentials{
		Username: conf.BrokerUsername,
		Password: conf.BrokerPassword,
	}
	brokerAPI := brokerapi.New(serviceBroker, logger, credentials)
	http.Handle("/", brokerAPI)
	port := "8080"
	if os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	}
	http.ListenAndServe(":"+port, nil)
}
