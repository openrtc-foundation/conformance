package capability

import (
	"context"
	"github.com/openrtc/conformance/harness"
)

func init() {
	CapabilityTests = append(CapabilityTests, harness.DefineTest(harness.ConformanceTest{
		ID: "CT-MEDIA-EXCHANGE-001", Title: "explicit exchange claims require identifiable content at the exact peer boundary", Suite: harness.Capability, Requirements: []string{"RFC §7.3:180", "RFC §14.1:1088–1098", "RFC §14.2:1108–1115"},
		Run: run(func(ctx context.Context, c *harness.TestContext) {
			target, ok := c.Target.(harness.MediaExchangeTarget)
			if !ok {
				must(c.NotApplicable("no explicit media exchange boundary claimed"))
			}
			f := target.MediaExchange()
			if f == nil {
				must(c.NotRun("claimed exchange lacks evidence fixture"))
			}
			boundaries := f.ClaimedBoundaries()
			if len(boundaries) == 0 {
				must(c.NotApplicable("no media exchange boundary claimed"))
			}
			for _, request := range boundaries {
				check(c, request.Boundary != "" && request.Source != "" && request.Kind != "" && request.Direction != "", "exchange claim omits exact boundary/source/kind/direction")
				observed := value(f.Exchange(ctx, request))
				if observed.Status == "unknown" {
					must(c.Unknown("independent exchange observation unavailable"))
				}
				check(c, observed.Status == "observed" && observed.Content != "" && observed.Evidence != "" && observed.Interval != "" && observed.Boundary == request.Boundary && observed.Source == request.Source && observed.Kind == request.Kind && observed.Direction == request.Direction, "sending metadata cannot prove exact claimed media receipt")
			}
		}),
	}))
}
