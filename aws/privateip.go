package aws

import (
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"golang.org/x/net/context"
)

// PrivateIP assigns an available private IP address to the calling EC2 node on
// nomination.
type PrivateIP struct {
	ec2         *ec2.EC2
	instanceID  string
	interfaceID string
	privateIPs  []string
}

// NewPrivateIP creates a new PrivateIP nominator.
func NewPrivateIP(region, instanceID string, privateIPs []string) (*PrivateIP, error) {
	if len(privateIPs) == 0 {
		return nil, errors.New("no private IPs provided")
	}
	interfaceID, err := EC2InterfaceID()
	if err != nil {
		return nil, err
	}
	return &PrivateIP{
		ec2:         ec2.New(&aws.Config{Region: aws.String(region)}),
		instanceID:  instanceID,
		interfaceID: interfaceID,
		privateIPs:  privateIPs,
	}, nil
}

// Nominate chooses and assigns a private IP address to the calling node.
func (pip *PrivateIP) Nominate(ctx context.Context, name string, size int, leaders map[string]string) (string, error) {
	assignedIPs := make(map[string]string, len(leaders))
	for name, ip := range leaders {
		assignedIPs[ip] = name
	}

	privateIP := ""
	for _, ip := range pip.privateIPs {
		if _, ok := assignedIPs[ip]; !ok {
			privateIP = ip
			break
		}
	}
	if privateIP == "" {
		return "", errors.New("no available private IP addresses")
	}

	truf := true
	_, err := pip.ec2.AssignPrivateIpAddresses(&ec2.AssignPrivateIpAddressesInput{
		NetworkInterfaceId: &pip.interfaceID,
		PrivateIpAddresses: []*string{&privateIP},
		AllowReassignment:  &truf,
	})
	if err != nil {
		err = fmt.Errorf("unable to assign IP %s to %s: %s", privateIP, pip.interfaceID, err)
	}
	return privateIP, err
}

// LeaderEvent processes a leader change event. We just discard them.
func (*PrivateIP) LeaderEvent(ctx context.Context, size int, leaders map[string]string) error {
	return nil
}
