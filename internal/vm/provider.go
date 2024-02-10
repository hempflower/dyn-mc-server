package vm

import "errors"

type VmProvider interface {
	Start() error
	Stop() error
	GetStatus() (VmStatus, error)
}

const (
	VM_STATUS_RUNNING = iota
	VM_STATUS_STOPPING
	VM_STATUS_STOPPED
	VM_STATUS_STARTING
)

type VmStatus struct {
	Status int
	Ip     string
}

func NewVmProvider(provider string, options map[string]interface{}) (VmProvider, error) {

	switch provider {
	case "aliyun":
		return NewAliyunProvider(
			options["accesskeyid"].(string),
			options["accesskeysecret"].(string),
			options["endpoint"].(string),
			options["instanceid"].(string)), nil
	default:
		return nil, errors.New("Unknown provider")
	}
}
