package aws

import (
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/service/ec2"
	"golang.org/x/net/context"
	"time"
)

// Interface assigns an available network interface to the calling EC2 node on
// nomination.
type Interface struct {
	ec2          *ec2.EC2
	instanceID   string
	interfaceIDs []string
}

// NewInterface creates a new Interface nominator.
func NewInterface(instanceID string, interfaceIDs []string) (*Interface, error) {
	if len(interfaceIDs) == 0 {
		return nil, errors.New("no interface IDs provided")
	}
	return &Interface{
		ec2:          ec2.New(nil),
		instanceID:   instanceID,
		interfaceIDs: interfaceIDs,
	}, nil
}

// Nominate chooses and assigns an interface to the calling node.
func (iface *Interface) Nominate(ctx context.Context, name string, size int, leaders map[string]string) (string, error) {
	assignedInterfaceIDs := make(map[string]string, len(leaders))
	for name, interfaceID := range leaders {
		assignedInterfaceIDs[interfaceID] = name
	}

	newInterfaceID := ""
	for _, interfaceID := range iface.interfaceIDs {
		if _, ok := assignedInterfaceIDs[interfaceID]; !ok {
			newInterfaceID = interfaceID
			break
		}
	}
	if newInterfaceID == "" {
		return "", errors.New("no available network interface")
	}

	err := iface.detach(ctx, newInterfaceID)
	if err == context.Canceled {
		return "", context.Canceled
	} else if err != nil {
		// we're going to try to attach even if the detach fails
		logger.Printf("unable to detach %s: %s", newInterfaceID, err)
	}

	debug.Printf("attaching interface %s to %s", newInterfaceID, iface.instanceID)
	var index int64 = 1
	_, err = iface.ec2.AttachNetworkInterface(&ec2.AttachNetworkInterfaceInput{
		InstanceId:         &iface.instanceID,
		NetworkInterfaceId: &newInterfaceID,
		DeviceIndex:        &index,
	})
	if err != nil {
		err = fmt.Errorf("unable to assign interface %s to %s: %s", newInterfaceID, iface.instanceID, err)
	}
	return newInterfaceID, err
}

// LeaderEvent processes a leader change event. We just discard them.
func (*Interface) LeaderEvent(ctx context.Context, size int, leaders map[string]string) error {
	return nil
}

// detach the network interface if it's currently attached
func (iface *Interface) detach(ctx context.Context, interfaceID string) (err error) {
	var atch *ec2.NetworkInterfaceAttachment
	for {
		if atch, err = iface.attachment(ctx, interfaceID); err != nil || atch == nil {
			// no attachment, nothing more to do
			debug.Printf("interface %s is detached", interfaceID)
			return
		}
		status := *atch.Status
		if status == ec2.AttachmentStatusAttached {
			// attached, exit loop and proceed
			debug.Printf("interface %s attached at %s", interfaceID, *atch.AttachmentId)
			break
		} else if status == ec2.AttachmentStatusDetached {
			// detached, nothing more to do
			debug.Printf("interface %s is detached", interfaceID)
			return
		}
		debug.Printf("interface %s is %s, waiting", interfaceID, status)

		// in the process of attaching or detaching, try again in a moment
		select {
		case <-time.After(100 * time.Millisecond):
		case <-ctx.Done():
			// operation was canceled, give up
			return context.Canceled
		}
	}

	debug.Printf("detaching interface %s at %s", interfaceID, *atch.AttachmentId)
	force := true
	_, err = iface.ec2.DetachNetworkInterface(&ec2.DetachNetworkInterfaceInput{
		AttachmentId: atch.AttachmentId,
		Force:        &force,
	})
	return err
}

func (iface *Interface) attachment(ctx context.Context, interfaceID string) (*ec2.NetworkInterfaceAttachment, error) {
	ch := make(chan interface{})
	go func() {
		defer close(ch)
		out, err := iface.ec2.DescribeNetworkInterfaces(&ec2.DescribeNetworkInterfacesInput{
			NetworkInterfaceIds: []*string{&interfaceID},
		})
		if err != nil {
			ch <- err
		}
		if len(out.NetworkInterfaces) > 0 {
			ch <- out.NetworkInterfaces[0].Attachment
		}
	}()

	select {
	case val, open := <-ch:
		if !open {
			return nil, nil
		}
		switch val := val.(type) {
		case *ec2.NetworkInterfaceAttachment:
			return val, nil
		case error:
			return nil, val
		default:
			panic(errors.New("unknown type in channel response"))
		}
	case <-ctx.Done():
		return nil, context.Canceled
	}
}
