package webhook_core

// NeedApplicationFlags this interface represent an object that need binding
// to application flags
type NeedApplicationFlags interface {
	// Bind this function will be called before application call `flags.Parse()`
	// to let this object bind its properties to application flags.
	// It may return an object that will be used to store temporary objects
	Bind() interface{}
	// CompleteBinding this will be called after application called `flag.Parse()`
	CompleteBinding(data interface{}) error
}

// AppFlagInitializationResponse a helper type
type AppFlagInitializationResponse struct {
	Instance NeedApplicationFlags
	Data     interface{}
}

// TryInitializeApplicationFlags try to initialize flags of specified instance of object
func TryInitializeApplicationFlags(instance interface{}) AppFlagInitializationResponse {
	needAppFlags, ok := instance.(NeedApplicationFlags)
	if !ok {
		return AppFlagInitializationResponse{
			Instance: nil,
			Data:     nil,
		}
	} else {
		return AppFlagInitializationResponse{
			Instance: needAppFlags,
			Data:     needAppFlags.Bind(),
		}
	}
}

// CompleteBinding complete binding after calling `flag.Parse()`
func CompleteBinding(o AppFlagInitializationResponse) error {
	if o.Instance == nil {
		return nil
	}

	return o.Instance.CompleteBinding(o.Data)
}
