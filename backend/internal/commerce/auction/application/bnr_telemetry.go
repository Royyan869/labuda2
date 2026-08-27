package application

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// BNR fail-open observability.
//
// When BNRStrikeChecker.Check() returns an error inside PlaceBid, the bid is
// allowed fail-open and this counter is incremented. The existing WARN log is
// preserved — this metric adds dashboard/alerting visibility without changing
// behavior.
//
// Cardinality: zero labels. A single error class (DB query failure during
// restriction check) is the only reason this counter fires.

var bnrRestrictionCheckFailedTotal = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: "labuda_auction",
	Name:      "bnr_restriction_check_failed_total",
	Help:      "Number of times the BNR restriction check failed and a bid was allowed fail-open. Non-zero values indicate DB connectivity or schema issues in the buyer_bnr_strikes query path.",
})

// RecordBNRCheckFailOpen increments the fail-open counter.
// Called from AuctionService.PlaceBid when BNRStrikeChecker.Check returns error.
func RecordBNRCheckFailOpen() {
	bnrRestrictionCheckFailedTotal.Inc()
}


