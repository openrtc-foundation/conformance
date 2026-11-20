package harness

// BindingPromiseTarget explicitly opts into bridge-specific promises. These
// checks are useful selftests but are outside universal RFC applicability.
// RFC §§2:13,5:27,14.2:1111; A.0:1174–1178.
type BindingPromiseTarget interface{ BindingPromises() []string }

func bindingSpecific(id string) bool {
	switch id {
	case "CT-CAP-001", "CT-CAP-002", "CT-CAP-003", "CT-CAP-004", "CT-CAP-005", "CT-CAP-006",
		"CT-EXT-001", "CT-EXT-002", "CT-EXT-003", "CT-MED-001", "CT-MED-002",
		"CT-CFG-001", "CT-CFG-002", "CT-CFG-003", "CT-CFG-004", "CT-CFG-005", "CT-CFG-006", "CT-CFG-007", "CT-CFG-008", "CT-CFG-009", "CT-PLC-005",
		"CT-REG-001", "CT-REG-002", "CT-REG-003", "CT-REG-004", "CT-REG-005",
		"CT-LIFE-001", "CT-LIFE-002", "CT-LIFE-003", "CT-LIFE-017", "CT-LIFE-026", "CT-LIFE-027", "CT-LIFE-028",
		"CT-EVT-004", "CT-EVT-005", "CT-EVT-006", "CT-CON-001", "CT-MOP-014", "CT-MOP-015", "CT-MOP-016":
		return true
	}
	return false
}
func bindingPromiseApplies(target ConformanceTarget, id string) bool {
	if !bindingSpecific(id) {
		return true
	}
	// Without a release claim, this is only a harness integration/selftest run.
	// Direct bindings never acquire implicit registry/ledger promises.
	if _, direct := target.(PublicBindingTarget); direct {
		return false
	}
	if _, declared := target.(ClaimTarget); !declared {
		return true
	}
	p, ok := target.(BindingPromiseTarget)
	return ok && contains(p.BindingPromises(), id)
}
