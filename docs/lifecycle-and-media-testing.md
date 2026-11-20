# Lifecycle and media test oracles

These principles describe what the suite must observe; the OpenRTC Core RFC remains normative. See [media contract assertions](../media-contracts.md) for detailed cases and RFC locators.

## Cancellation and cleanup

Binding-documented finite cancellation, deadline-expiry, termination-fence, and release-invocation bounds are distinct from the harness watchdog. Valid cancellation races local completion without waiting for native abort support. Cancelling a Go Result waiter is not operation cancellation or settlement. Tests check portable results and stable repeated reads while provider completion remains blocked. Late admission cannot resurrect cancelled participation.

Local leave or dispose completion establishes local terminal participation and initiation of owned release, not remote release. A cleanup request acknowledgment is distinct from independent release evidence. Pending, failed, and unknown cleanup identifies its resource or bounded scope; an established failure retains a safe cleanup error separately from the initiating failure. Later outcomes may evolve through documented retention. Repeated release may retry idempotently; native SDK call counts do not define conformance.

## Observations and effects

Snapshots preserve coverage and completeness for local and receive-source populations. Omission is not unknown, and scope alone is not completeness. Call publications belong to the Call; granted sources retain their actual owner. Independent publication mute and setSending suppression combine by union, while source disable remains independent. Additional effects require bounded disclosure and explicit pre-effect acceptance; acceptance cannot waive target postconditions, authority, or consent.

Safe portable failures use the ten RFC §9 codes, established reasons, and honest none/partial/unknown outcomes. Default errors, formatting, snapshots, events, logs, and serialization exclude raw provider payloads, stacks, and secret material. Unrestricted diagnostics require an explicit request within an authorized boundary.
