package vm

import (
	openapi "github.com/alibabacloud-go/darabonba-openapi/client"
	ecs "github.com/alibabacloud-go/ecs-20140526/v4/client"
	"github.com/alibabacloud-go/tea/tea"
)

type AliyunProvider struct {
	accessId     string
	accessSecret string
	ecsClient    *ecs.Client
	instanceId   string
}

func NewAliyunProvider(accessId, accessSecret, endpoint, instanceId string) *AliyunProvider {
	config := &openapi.Config{
		AccessKeyId:     &accessId,
		AccessKeySecret: &accessSecret,
	}
	config.Endpoint = tea.String(endpoint)
	ecsClient, _ := ecs.NewClient(config)

	return &AliyunProvider{
		accessId:     accessId,
		accessSecret: accessSecret,
		ecsClient:    ecsClient,
		instanceId:   instanceId,
	}
}

func (p *AliyunProvider) Start() error {

	startInstanceRequest := &ecs.StartInstanceRequest{}
	startInstanceRequest.SetInstanceId(p.instanceId)

	_, err := p.ecsClient.StartInstance(startInstanceRequest)
	if err != nil {
		return err
	}

	return nil
}

func (p *AliyunProvider) Stop() error {

	stopInstanceRequest := &ecs.StopInstanceRequest{}

	stopInstanceRequest.SetInstanceId(p.instanceId)
	// 使用节省费用的方式停止实例
	stopInstanceRequest.SetStoppedMode("StopCharging")

	_, err := p.ecsClient.StopInstance(stopInstanceRequest)
	if err != nil {
		return err
	}

	return nil
}

func (p *AliyunProvider) GetStatus() (VmStatus, error) {
	describeInstanceAttributeRequest := &ecs.DescribeInstanceAttributeRequest{}
	describeInstanceAttributeRequest.SetInstanceId(p.instanceId)

	resp, err := p.ecsClient.DescribeInstanceAttribute(describeInstanceAttributeRequest)
	if err != nil {
		return VmStatus{}, err
	}

	status := VM_STATUS_STOPPED
	if *resp.Body.Status == "Running" {
		status = VM_STATUS_RUNNING
	} else if *resp.Body.Status == "Starting" {
		status = VM_STATUS_STARTING
	} else if *resp.Body.Status == "Stopping" {
		status = VM_STATUS_STOPPING
	}

	publicIp := ""
	if status == VM_STATUS_RUNNING {
		publicIp = *resp.Body.PublicIpAddress.IpAddress[0]
	}

	return VmStatus{
		Status: status,
		Ip:     publicIp,
	}, nil
}
