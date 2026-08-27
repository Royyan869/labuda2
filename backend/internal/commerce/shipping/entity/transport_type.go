package entity

// TransportType represents the general transportation category.
type TransportType string

const (
	// TransportTrain represents train-based shipping.
	TransportTrain TransportType = "train"
	// TransportBus represents bus-based shipping.
	TransportBus TransportType = "bus"
	// TransportTravel represents travel/shuttle-based shipping.
	TransportTravel TransportType = "travel"
	// TransportPlane represents air freight shipping.
	TransportPlane TransportType = "plane"
	// TransportCustom represents other/custom transport types.
	TransportCustom TransportType = "custom"
)


