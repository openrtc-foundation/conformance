package openrtc

// Typed keys encode the extension-map relationship in Go, whose interface
// methods cannot have type parameters. Acquisition retains the owner's gates.
type ClientExtensionKey[T any] struct{ name ExtensionKey }
type SessionExtensionKey[T any] struct{ name ExtensionKey }
type ServerExtensionKey[T any] struct{ name ExtensionKey }

var (
	CallingKey                = ClientExtensionKey[CallingExtension]{"calling"}
	DevicesKey                = ClientExtensionKey[MediaDeviceManager]{"devices"}
	IndependentPublicationKey = SessionExtensionKey[IndependentPublicationControl]{"independent-publication"}
	ManualSubscriptionKey     = SessionExtensionKey[ManualSubscriptionControl]{"manual-subscription"}
	StatisticsKey             = SessionExtensionKey[MediaStatistics]{"statistics"}
	RecordingKey              = SessionExtensionKey[RecordingExtension]{"recording"}
	MessagingKey              = SessionExtensionKey[MessagingExtension]{"messaging"}
	ScreenSharingKey          = SessionExtensionKey[ScreenSharingExtension]{"screen-sharing"}
	EncryptionKey             = SessionExtensionKey[EncryptionExtension]{"encryption"}
	SessionAdministrationKey  = SessionExtensionKey[SessionAdministrationExtension]{"session-administration"}
	ServerAdministrationKey   = ServerExtensionKey[SessionAdministrationExtension]{"session-administration"}
	ServerRecordingKey        = ServerExtensionKey[RecordingExtension]{"recording"}
)

func ClientExtension[T any](owner RTCClient, key ClientExtensionKey[T]) T {
	return owner.Extension(key.name).(T)
}
func SessionExtension[T any](owner RTCSessionBase, key SessionExtensionKey[T]) T {
	return owner.Extension(key.name).(T)
}
func ServerExtension[T any](owner RTCServerAdministration, key ServerExtensionKey[T]) T {
	return owner.Extension(key.name).(T)
}
