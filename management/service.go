package management

import (
	"github.com/RobotsAndPencils/buford/payload"
	"github.com/RobotsAndPencils/buford/push"
	"github.com/micromdm/dep"
	"github.com/micromdm/micromdm/device"
	"github.com/micromdm/micromdm/workflow"
	"github.com/pkg/errors"
)

// ErrNotFound ...
var ErrNotFound = errors.New("not found")

// Service is the interface that provides methods for managing devices
type Service interface {
	// profiles
	AddProfile(prf *workflow.Profile) (*workflow.Profile, error)
	Profiles() ([]workflow.Profile, error)
	Profile(uuid string) (*workflow.Profile, error)
	DeleteProfile(uuid string) error
	// workflows
	AddWorkflow(wf *workflow.Workflow) (*workflow.Workflow, error)
	Workflows() ([]workflow.Workflow, error)

	// Devices
	Devices() ([]device.Device, error)
	Device(uuid string) (*device.Device, error)
	// AssignWorkflow assigns a workflow to a device
	AssignWorkflow(deviceUUID, workflowUUID string) error

	// push sends a new push notification to the device
	// returning the notification ID
	Push(deviceUDID string) (string, error)

	// FetchDEPDevices updates the device datastore with devices from DEP
	FetchDEPDevices() error
}

// NewService creates a management service
func NewService(ds device.Datastore, ws workflow.Datastore, dc dep.Client, ps *push.Service) Service {
	return &service{
		devices:   ds,
		depClient: dc,
		workflows: ws,
		pushsvc:   ps,
	}
}

type service struct {
	depClient dep.Client
	devices   device.Datastore
	workflows workflow.Datastore
	pushsvc   *push.Service
}

func (svc service) Push(deviceUDID string) (string, error) {
	dev, err := svc.devices.GetDeviceByUDID(deviceUDID,
		[]string{"device_uuid",
			"apple_push_magic",
			"apple_mdm_token",
		}...,
	)
	if err != nil {
		return "", err
	}

	p := payload.MDM{Token: dev.PushMagic}
	valid := push.IsDeviceTokenValid(dev.Token)
	if !valid {
		return "", errors.New("invalid push token")
	}
	return svc.pushsvc.Push(dev.Token, nil, p)
}

func (svc service) AddProfile(prf *workflow.Profile) (*workflow.Profile, error) {
	return svc.workflows.CreateProfile(prf)
}

func (svc service) Profiles() ([]workflow.Profile, error) {
	return svc.workflows.Profiles()
}

// Profile returns a single profile given an UUID
func (svc service) Profile(uuid string) (*workflow.Profile, error) {
	profiles, err := svc.workflows.Profiles(workflow.ProfileUUID{UUID: uuid})
	if err != nil {
		return nil, err
	}
	if len(profiles) == 0 {
		return nil, ErrNotFound
	}
	pf := profiles[0]
	return &pf, nil
}

func (svc service) DeleteProfile(uuid string) error {
	pr, err := svc.Profile(uuid) // get profile from datastore
	if err != nil {
		return err
	}
	err = svc.workflows.DeleteProfile(pr)
	if err != nil {
		return err
	}
	return nil
}

func (svc service) FetchDEPDevices() error {
	fetched, err := svc.depClient.FetchDevices(dep.Limit(100))
	if err != nil {
		return errors.Wrap(err, "management: dep fetch")
	}
	for _, d := range fetched.Devices {
		dev := device.NewFromDEP(d)
		_, err := svc.devices.New("fetch", dev)
		if err != nil {
			return errors.Wrap(err, "management: dep fetch")
		}
	}
	return nil
}

// workflows svc
func (svc service) AddWorkflow(wf *workflow.Workflow) (*workflow.Workflow, error) {
	return svc.workflows.CreateWorkflow(wf)
}

func (svc service) Workflows() ([]workflow.Workflow, error) {
	return svc.workflows.Workflows()
}

// devices
func (svc service) Devices() ([]device.Device, error) {
	return svc.devices.Devices()
}

func (svc service) Device(uuid string) (*device.Device, error) {
	devices, err := svc.devices.Devices(device.UUID{UUID: uuid})
	if err != nil {
		return nil, err
	}
	if len(devices) == 0 {
		return nil, ErrNotFound
	}
	dev := devices[0]
	return &dev, nil
}

func (svc service) AssignWorkflow(deviceUUID, workflowUUID string) error {
	dev, err := svc.devices.GetDeviceByUUID(deviceUUID,
		[]string{"device_uuid"}...,
	)
	if err != nil {
		return errors.Wrap(err, "management: assign workflow")
	}
	dev.Workflow = workflowUUID
	return svc.devices.Save("assignWorkflow", dev)
}
