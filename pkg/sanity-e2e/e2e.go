/*
Copyright 2017 Kubernetes Authornc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sanitye2e

import (
	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	context "golang.org/x/net/context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func isControllerCapabilitySupported(
	cc csi.ControllerClient,
	capType csi.ControllerServiceCapability_RPC_Type,
) bool {

	caps, err := cc.ControllerGetCapabilities(
		context.Background(),
		&csi.ControllerGetCapabilitiesRequest{})
	Expect(err).NotTo(HaveOccurred())
	Expect(caps).NotTo(BeNil())
	Expect(caps.GetCapabilities()).NotTo(BeNil())

	for _, cap := range caps.GetCapabilities() {
		Expect(cap.GetRpc()).NotTo(BeNil())
		if cap.GetRpc().GetType() == capType {
			return true
		}
	}
	return false
}

func isNodeCapabilitySupported(nc csi.NodeClient,
	capType csi.NodeServiceCapability_RPC_Type,
) bool {

	caps, err := nc.NodeGetCapabilities(
		context.Background(),
		&csi.NodeGetCapabilitiesRequest{})
	Expect(err).NotTo(HaveOccurred())
	Expect(caps).NotTo(BeNil())
	Expect(caps.GetCapabilities()).NotTo(BeNil())

	for _, cap := range caps.GetCapabilities() {
		Expect(cap.GetRpc()).NotTo(BeNil())
		if cap.GetRpc().GetType() == capType {
			return true
		}
	}
	return false
}

var _ = Describe("E2E Tests", func() {
	var (
		cc                         csi.ControllerClient
		nc                         csi.NodeClient
		controllerPublishSupported bool
		nodeStageSupported         bool
	)

	BeforeEach(func() {
		cc = csi.NewControllerClient(conn)
		nc = csi.NewNodeClient(conn)
		controllerPublishSupported = isControllerCapabilitySupported(
			cc,
			csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME)
		nodeStageSupported = isNodeCapabilitySupported(nc, csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME)
	})

	It("should complete create/delete successfully (no optional values added)", func() {
		// Create Volume First
		By("creating a volume")
		name := "sanity"
		vol, err := cc.CreateVolume(
			context.Background(),
			&csi.CreateVolumeRequest{
				Name: name,
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
			})

		Expect(err).NotTo(HaveOccurred())
		Expect(vol).NotTo(BeNil())
		Expect(vol.GetVolume()).NotTo(BeNil())
		Expect(vol.GetVolume().GetId()).NotTo(BeEmpty())

		// Delete Volume
		By("deleting a volume")
		_, err = cc.DeleteVolume(
			context.Background(),
			&csi.DeleteVolumeRequest{
				VolumeId: vol.GetVolume().GetId(),
			})
		Expect(err).NotTo(HaveOccurred())
	})

	It("should complete full lifecycle of publish volume successfully (no optional values added)", func() {

		// Create Volume First
		By("creating a single node writer volume")
		name := "sanity"
		vol, err := cc.CreateVolume(
			context.Background(),
			&csi.CreateVolumeRequest{
				Name: name,
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
			})
		Expect(err).NotTo(HaveOccurred())
		Expect(vol).NotTo(BeNil())
		Expect(vol.GetVolume()).NotTo(BeNil())
		Expect(vol.GetVolume().GetId()).NotTo(BeEmpty())

		By("getting a node id")
		nid, err := nc.NodeGetId(
			context.Background(),
			&csi.NodeGetIdRequest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(nid).NotTo(BeNil())
		Expect(nid.GetNodeId()).NotTo(BeEmpty())
		var conpubvol *csi.ControllerPublishVolumeResponse
		if controllerPublishSupported {
			By("controller publishing volume")
			conpubvol, err = cc.ControllerPublishVolume(
				context.Background(),
				&csi.ControllerPublishVolumeRequest{
					VolumeId: vol.GetVolume().GetId(),
					NodeId:   nid.GetNodeId(),
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
					Readonly: false,
				})
			Expect(err).NotTo(HaveOccurred())
			Expect(conpubvol).NotTo(BeNil())
		}
		// NodeStageVolume
		if nodeStageSupported {
			By("node staging volume")
			nodeStageVolReq := &csi.NodeStageVolumeRequest{
				VolumeId: vol.GetVolume().GetId(),
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				StagingTargetPath: stagingTargetPath,
			}
			if controllerPublishSupported {
				nodeStageVolReq.PublishInfo = conpubvol.GetPublishInfo()
			}
			nodestagevol, err := nc.NodeStageVolume(
				context.Background(), nodeStageVolReq)
			Expect(err).NotTo(HaveOccurred())
			Expect(nodestagevol).NotTo(BeNil())
		}
		// NodePublishVolume
		By("publishing the volume on a node")
		nodepubvolRequest := &csi.NodePublishVolumeRequest{
			VolumeId:   vol.GetVolume().GetId(),
			TargetPath: csiTargetPath,
			VolumeCapability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{},
				},
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
				},
			},
		}
		if nodeStageSupported {
			nodepubvolRequest.StagingTargetPath = stagingTargetPath
		}
		if controllerPublishSupported {
			nodepubvolRequest.PublishInfo = conpubvol.GetPublishInfo()
		}
		nodepubvol, err := nc.NodePublishVolume(context.Background(), nodepubvolRequest)
		Expect(err).NotTo(HaveOccurred())
		Expect(nodepubvol).NotTo(BeNil())

		// NodeUnpublishVolume
		By("cleaning up calling nodeunpublish")
		nodeunpubvol, err := nc.NodeUnpublishVolume(
			context.Background(),
			&csi.NodeUnpublishVolumeRequest{
				VolumeId:   vol.GetVolume().GetId(),
				TargetPath: csiTargetPath,
			})
		Expect(err).NotTo(HaveOccurred())
		Expect(nodeunpubvol).NotTo(BeNil())

		if controllerPublishSupported {
			By("cleaning up calling controllerunpublishing the volume")
			nodeunpubvol, err := nc.NodeUnpublishVolume(
				context.Background(),
				&csi.NodeUnpublishVolumeRequest{
					VolumeId:   vol.GetVolume().GetId(),
					TargetPath: csiTargetPath,
				})
			Expect(err).NotTo(HaveOccurred())
			Expect(nodeunpubvol).NotTo(BeNil())
		}

		By("cleaning up deleting the volume")
		_, err = cc.DeleteVolume(
			context.Background(),
			&csi.DeleteVolumeRequest{
				VolumeId: vol.GetVolume().GetId(),
			})
		Expect(err).NotTo(HaveOccurred())
	})

	It("should validate volume capabilites of created volume successfully (no optional values added)", func() {
		// Create Volume First
		By("creating a single node writer volume")
		name := "sanity"
		vol, err := cc.CreateVolume(
			context.Background(),
			&csi.CreateVolumeRequest{
				Name: name,
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
			})

		Expect(err).NotTo(HaveOccurred())
		Expect(vol).NotTo(BeNil())
		Expect(vol.GetVolume()).NotTo(BeNil())
		Expect(vol.GetVolume().GetId()).NotTo(BeEmpty())

		// ValidateVolumeCapabilities
		By("validating volume capabilities")
		valivolcap, err := cc.ValidateVolumeCapabilities(
			context.Background(),
			&csi.ValidateVolumeCapabilitiesRequest{
				VolumeId: vol.GetVolume().GetId(),
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
			})
		Expect(err).NotTo(HaveOccurred())
		Expect(valivolcap).NotTo(BeNil())
		Expect(valivolcap.GetSupported()).To(BeTrue())

		By("cleaning up deleting the volume")
		_, err = cc.DeleteVolume(
			context.Background(),
			&csi.DeleteVolumeRequest{
				VolumeId: vol.GetVolume().GetId(),
			})
		Expect(err).NotTo(HaveOccurred())
	})
})
