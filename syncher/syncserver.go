// Copyright (c) 2018 Tigera, Inc. All rights reserved.

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

package syncher

import (
	"context"

	"github.com/projectcalico/app-policy/policystore"
	"github.com/projectcalico/app-policy/proto"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type syncClient struct {
	target   string
	dialOpts []grpc.DialOption
}

func NewClient(target string, opts []grpc.DialOption) *syncClient {
	return &syncClient{target: target, dialOpts: opts}
}

func (s *syncClient) Sync(cxt context.Context, store *policystore.PolicyStore) {
	// TODO: Handle connection errors more gracefully than Fatal.
	conn, err := grpc.Dial(s.target, s.dialOpts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()
	client := proto.NewPolicySyncClient(conn)
	stream, err := client.Sync(cxt, &proto.SyncRequest{})
	if err != nil {
		log.Fatal("failed to Sync with server: %v", err)
	}
	for {
		update, err := stream.Recv()
		if err != nil {
			log.Fatal("connection to Policy Sync server broken: %v", err)
		}
		store.Write(func(ps *policystore.PolicyStore) { processUpdate(ps, update) })
	}
	// Note that as written, this function will never return. It only ends when the connection is torn down, which
	// terminates the entire program.
}

// Update the PolicyStore with the information passed over the Sync API.
func processUpdate(store *policystore.PolicyStore, update *proto.ToDataplane) {
	switch payload := update.Payload.(type) {
	case *proto.ToDataplane_InSync:
		processInSync(store, payload.InSync)
	case *proto.ToDataplane_IpsetUpdate:
		processIPSetUpdate(store, payload.IpsetUpdate)
	case *proto.ToDataplane_IpsetDeltaUpdate:
		processIPSetDeltaUpdate(store, payload.IpsetDeltaUpdate)
	case *proto.ToDataplane_IpsetRemove:
		processIPSetRemove(store, payload.IpsetRemove)
	case *proto.ToDataplane_ActiveProfileUpdate:
		processActiveProfileUpdate(store, payload.ActiveProfileUpdate)
	case *proto.ToDataplane_ActiveProfileRemove:
		processActiveProfileRemove(store, payload.ActiveProfileRemove)
	case *proto.ToDataplane_ActivePolicyUpdate:
		processActivePolicyUpdate(store, payload.ActivePolicyUpdate)
	case *proto.ToDataplane_ActivePolicyRemove:
		processActivePolicyRemove(store, payload.ActivePolicyRemove)
	case *proto.ToDataplane_WorkloadEndpointUpdate:
		processWorkloadEndpointUpdate(store, payload.WorkloadEndpointUpdate)
	case *proto.ToDataplane_WorkloadEndpointRemove:
		processWorkloadEndpointRemove(store, payload.WorkloadEndpointRemove)
	}
}

func processInSync(store *policystore.PolicyStore, inSync *proto.InSync) {
	// TODO (spikecurtis): disallow requests until policy is synced?
	return
}

func processIPSetUpdate(store *policystore.PolicyStore, update *proto.IPSetUpdate) {
	s := store.IPSetByID[update.Id]
	if s == nil {
		s = policystore.NewIPSet(update.Type)
	}
	for _, addr := range update.Members {
		s.AddString(addr)
	}
}

func processIPSetDeltaUpdate(store *policystore.PolicyStore, update *proto.IPSetDeltaUpdate) {
	s := store.IPSetByID[update.Id]
	if s == nil {
		log.Fatalf("Unknown IPSet id: %v", update.Id)
	}
	for _, addr := range update.AddedMembers {
		s.AddString(addr)
	}
	for _, addr := range update.RemovedMembers {
		s.RemoveString(addr)
	}
}

func processIPSetRemove(store *policystore.PolicyStore, update *proto.IPSetRemove) {
	delete(store.IPSetByID, update.Id)
}

func processActiveProfileUpdate(store *policystore.PolicyStore, update *proto.ActiveProfileUpdate) {
	if update.Id == nil {
		log.Fatal("got ActiveProfileUpdate with nil ProfileID")
	}
	store.ProfileByID[*update.Id] = update.Profile
}

func processActiveProfileRemove(store *policystore.PolicyStore, update *proto.ActiveProfileRemove) {
	if update.Id == nil {
		log.Fatal("got ActiveProfileRemove with nil ProfileID")
	}
	delete(store.ProfileByID, *update.Id)
}

func processActivePolicyUpdate(store *policystore.PolicyStore, update *proto.ActivePolicyUpdate) {
	if update.Id == nil {
		log.Fatal("got ActivePolicyUpdate with nil PolicyID")
	}
	store.PolicyByID[*update.Id] = update.Policy
}

func processActivePolicyRemove(store *policystore.PolicyStore, update *proto.ActivePolicyRemove) {
	if update.Id == nil {
		log.Fatal("got ActivePolicyRemove with nil PolicyID")
	}
	delete(store.PolicyByID, *update.Id)
}

func processWorkloadEndpointUpdate(store *policystore.PolicyStore, update *proto.WorkloadEndpointUpdate) {
	// TODO: check the WorkloadEndpointID?
	log.WithFields(log.Fields{
		"orchestrator_id": update.GetId().GetOrchestratorId(),
		"workload_id":     update.GetId().GetWorkloadId(),
		"endpoint_id":     update.GetId().GetEndpointId(),
	}).Info("Got WorkloadEndpointUpdate")
	store.Endpoint = update.Endpoint
}

func processWorkloadEndpointRemove(store *policystore.PolicyStore, update *proto.WorkloadEndpointRemove) {
	// TODO: maybe this isn't required, because removing the endpoint means shutting down the pod?
	log.WithFields(log.Fields{
		"orchestrator_id": update.GetId().GetOrchestratorId(),
		"workload_id":     update.GetId().GetWorkloadId(),
		"endpoint_id":     update.GetId().GetEndpointId(),
	}).Warning("Got WorkloadEndpointRemove")
	store.Endpoint = nil
}
