package flags


// EventingEnvironmentFlags holds the e2e flags needed only by the eventing repo.
type EventingEnvironmentFlags struct {
	BrokerClass string
	Channels
	Sources
}
