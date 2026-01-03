package main

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/common"
	"github.com/oracle/oci-go-sdk/core"
	"github.com/oracle/oci-go-sdk/identity"
	"github.com/sirupsen/logrus"
)

func (s *Configuration) createInstancesInAvailabilityZone(ctx context.Context, domains identity.ListAvailabilityDomainsResponse) {
	for _, domain := range domains.Items {
		logrus.Infof("Attempting to create new instance in domain %v", *domain.Name)
		logrus.Debugf("Current Range: [Attempt Interval: %v, Sleep Time between Zones: %v]", s.CreateIntervalSeconds, s.ZoneIntervalSeconds)
		s.CreateInstance(ctx, domain)
		time.Sleep(time.Duration(s.ZoneIntervalSeconds) * time.Second)
	}
}

func (s *Configuration) CreateInstance(ctx context.Context, domain identity.AvailabilityDomain) {
	trueVar := true
	ocpuVar := float32(s.CPUs)
	volumeVar := int64(s.VolumeGb)

	req := core.LaunchInstanceRequest{
		LaunchInstanceDetails: core.LaunchInstanceDetails{
			AvailabilityDomain: domain.Name,
			CompartmentId:      &s.TenancyId,
			Shape:              &s.Shape,
			CreateVnicDetails: &core.CreateVnicDetails{
				AssignPublicIp: &trueVar,
				SubnetId:       &s.SubnetId,
			},
			DisplayName: &s.DisplayName,
			Metadata: map[string]string{
				"ssh_authorized_keys": s.SSHKey,
			},
			ShapeConfig: &core.LaunchInstanceShapeConfigDetails{
				Ocpus: &(ocpuVar),
			},
			SourceDetails: &core.InstanceSourceViaImageDetails{
				ImageId:             &s.ImageId,
				BootVolumeSizeInGBs: &volumeVar,
			},
			IsPvEncryptionInTransitEnabled: &trueVar,
		},
	}

	resp, err := s.client.LaunchInstance(ctx, req)
	if err == nil {
		logrus.Errorf("Generated instance in availability zone %v, took %v", domain.Id, time.Since(s.started).String())
		logrus.Errorf("[########################] SUCCESS!!!! [########################]")
		os.Exit(0)
	}

	if !strings.Contains(err.Error(), "Out of host capacity") {
		logrus.Infof("Received error from api: %v; resp: %v, req: %v", err, resp, req)
	} else {
		logrus.Debugf("OutOfHostCapacity Error: %v", err)
	}

	if strings.Contains(err.Error(), "error:TooManyRequests") {
		logrus.Infof("Received too many requests, increasing request time between instances, current interval %v", time.Duration(s.ZoneIntervalSeconds).String())
		logrus.Debugf("Too Many Requests Error: %v", err)
		if time.Duration(s.ZoneIntervalSeconds) <= 20*time.Second {
			s.ZoneIntervalSeconds += 1
		}
	}

	// SUCCESS CASE
	if err == nil {
		logrus.Infof("[########################] SUCCESS!!!! [########################]")
		logrus.Infof("Instance ID: %v", *resp.Instance.Id)
		os.Exit(0)
	}

	// ERROR CASE - Better Logging Starts Here
	if serviceErr, ok := common.IsServiceError(err); ok {
		// This captures the specific Oracle Error
		logrus.Errorf("Oracle Error Code: %s", serviceErr.GetCode())
		logrus.Errorf("Oracle Message: %s", serviceErr.GetMessage())
		logrus.Errorf("Request ID: %s", serviceErr.GetOpcRequestID())
	} else {
		// This captures network or internal Go errors
		logrus.Errorf("Internal/Network Error: %v", err)
	}
}

func (s *Configuration) ListDomains(ctx context.Context) (identity.ListAvailabilityDomainsResponse, error) {
	domainClient, err := identity.NewIdentityClientWithConfigurationProvider(s)
	if err != nil {
		panic(err)
	}

	return domainClient.ListAvailabilityDomains(ctx, identity.ListAvailabilityDomainsRequest{
		CompartmentId:   &s.TenancyId,
		RequestMetadata: common.RequestMetadata{},
	})
}
