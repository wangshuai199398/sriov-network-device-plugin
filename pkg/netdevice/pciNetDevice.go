// Copyright 2018 Intel Corp. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package netdevice

import (
	"github.com/golang/glog"
	"github.com/jaypipes/ghw"

	"github.com/k8snetworkplumbingwg/sriov-network-device-plugin/pkg/devices"
	"github.com/k8snetworkplumbingwg/sriov-network-device-plugin/pkg/infoprovider"
	"github.com/k8snetworkplumbingwg/sriov-network-device-plugin/pkg/types"
	"github.com/k8snetworkplumbingwg/sriov-network-device-plugin/pkg/utils"
)

// pciNetDevice extends HostDevice and embedds GenPciDevice and GenNetDevice
type pciNetDevice struct {
	types.HostDevice
	devices.GenPciDevice
	devices.GenNetDevice
	vdpaDev types.VdpaDevice
}

// NewPciNetDevice returns an instance of PciNetDevice interface
func NewPciNetDevice(dev *ghw.PCIDevice,
	rFactory types.ResourceFactory, rc *types.ResourceConfig, selectorIndex int) (types.PciNetDevice, error) {
	var vdpaDev types.VdpaDevice
	var nf *types.NetDeviceSelectors

	driverName, err := utils.GetDriverName(dev.Address)
	if err != nil {
		glog.Infof("can not get driver for device %v, err: %v", dev.Address, err)
		return nil, err
	}

	infoProviders := rFactory.GetDefaultInfoProvider(dev.Address, driverName)
	if rc.AdditionalInfo != nil {
		glog.Infof("AdditionalInfo: %v", rc.AdditionalInfo)
		infoProviders = append(infoProviders, infoprovider.NewExtraInfoProvider(dev.Address, rc.AdditionalInfo))
	}

	isRdma := false
	ok := false
	if selectorIndex >= 0 && selectorIndex < len(rc.SelectorObjs) {
		nf, ok = rc.SelectorObjs[selectorIndex].(*types.NetDeviceSelectors)
	}
	if ok {
		// Add InfoProviders based on Selector data
		if nf.VdpaType != "" {
			vdpaDev = rFactory.GetVdpaDevice(dev.Address)
			if vdpaDev == nil {
				glog.Warningf("No vDPA device found for device %s", dev.Address)
			} else {
				infoProviders = append(infoProviders, infoprovider.NewVdpaInfoProvider(nf.VdpaType, vdpaDev))
			}
		} else if nf.IsRdma {
			rdmaSpec := rFactory.GetRdmaSpec(types.NetDeviceType, dev.Address)
			if rdmaSpec.IsRdma() {
				isRdma = true
				infoProviders = append(infoProviders, infoprovider.NewRdmaInfoProvider(rdmaSpec))
			} else {
				glog.Warningf("RDMA resources for %s not found. Are RDMA modules loaded?", dev.Address)
			}
		}
		if nf.NeedVhostNet {
			if infoprovider.VhostNetDeviceExist() {
				infoProviders = append(infoProviders, infoprovider.NewVhostNetInfoProvider())
			} else {
				glog.Errorf("GetDeviceSpecs(): vhost-net is required in the configuration but /dev/vhost-net doesn't exist")
			}
		}
	}

	hostDev, err := devices.NewHostDeviceImpl(dev, dev.Address, rFactory, rc, infoProviders)
	if err != nil {
		glog.Infof("NewHostDeviceImpl err: %v", err)
		return nil, err
	}

	pciDev, err := devices.NewGenPciDevice(dev)
	if err != nil {
		glog.Infof("NewGenPciDevice err: %v", err)
		return nil, err
	}

	netDev, err := devices.NewGenNetDevice(dev.Address, types.NetDeviceType, isRdma)
	if err != nil {
		glog.Infof("NewGenNetDevice err: %v", err)
		return nil, err
	}

	glog.Infof("get pciNetDevice hostDev: %v, pciDev: %v, netDev: %v, vdpaDev: %v", hostDev, pciDev, netDev, vdpaDev)
	return &pciNetDevice{
		HostDevice:   hostDev,
		GenPciDevice: *pciDev,
		GenNetDevice: *netDev,
		vdpaDev:      vdpaDev,
	}, nil
}

func (nd *pciNetDevice) GetDDPProfiles() string {
	pciAddr := nd.GetPciAddr()
	ddpProfile, err := utils.GetDDPProfiles(pciAddr)
	if err != nil {
		glog.Infof("GetDDPProfiles(): unable to get ddp profiles for device %s : %q", pciAddr, err)
		return ""
	}
	return ddpProfile
}

func (nd *pciNetDevice) GetVdpaDevice() types.VdpaDevice {
	return nd.vdpaDev
}
