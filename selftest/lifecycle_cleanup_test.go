package selftest

import "testing"

// RFC §8.6:506–510; observed remote progress remains separate from local release.
func TestPartialCleanupRetainedThroughDeclaredWindow(t *testing.T) {
	checkLifecycleOracle(t, "CT-CLEAN-001", FakeDefects{CleanupRetentionLost: true})
}
func TestRepeatedReleaseWhileRemoteCleanupUnresolved(t *testing.T) {
	checkLifecycleOracle(t, "CT-CLEAN-002", FakeDefects{RepeatedReleaseResurrects: true})
}
func TestCleanupAcknowledgementIsNotRemoteRelease(t *testing.T) {
	checkLifecycleOracle(t, "CT-CLEAN-003", FakeDefects{CleanupAcknowledgedAsReleased: true})
}
func TestObsoleteCallbacksAfterDisposedOwner(t *testing.T) {
	checkLifecycleOracle(t, "CT-STALE-001", FakeDefects{StaleCallbacksCommit: true})
	t.Run("duplicate allocation", func(t *testing.T) { checkLifecycleOracle(t, "CT-STALE-001", FakeDefects{DuplicateAllocation: true}) })
}
