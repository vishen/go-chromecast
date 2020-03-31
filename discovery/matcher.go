package discovery

// DeviceMatcher allows to specicy which device should be accepted
type DeviceMatcher func(*Device) bool

// WithName matches a device by its name
func WithName(name string) DeviceMatcher {
	return func(device *Device) bool {
		return device != nil && device.Name() == name
	}
}

// WithID matches a device by its id
func WithID(id string) DeviceMatcher {
	return func(device *Device) bool {
		return device != nil && device.ID() == id
	}
}

// WithType matches a device by its type
func WithType(t string) DeviceMatcher {
	return func(device *Device) bool {
		return device != nil && device.Type() == t
	}
}

func matchAll(matchers ...DeviceMatcher) DeviceMatcher {
	return func(device *Device) bool {
		for _, m := range matchers {
			if !m(device) {
				return false
			}
		}
		return true
	}
}
