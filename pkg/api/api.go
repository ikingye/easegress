/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package api

import (
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/megaease/easegress/pkg/logger"
	yaml "gopkg.in/yaml.v2"
)

func aboutText() string {
	return fmt.Sprintf(`Copyright © 2017 - %d MegaEase(https://megaease.com). All rights reserved.
Powered by open-source software: Etcd(https://etcd.io), Apache License 2.0.
`, time.Now().Year())
}

const (
	// APIPrefix is the prefix of api.
	APIPrefix = "/apis/v1"

	lockKey = "/config/lock"

	// ConfigVersionKey is the key of header for config version.
	ConfigVersionKey = "X-Config-Version"
)

var (
	apisMutex      = sync.Mutex{}
	apis           = make(map[string]*APIGroup)
	apisChangeChan = make(chan struct{}, 10)
)

type apisbyOrder []*APIGroup

func (a apisbyOrder) Less(i, j int) bool { return a[i].Group < a[j].Group }
func (a apisbyOrder) Len() int           { return len(a) }
func (a apisbyOrder) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

// RegisterAPIs registers global admin APIs.
func RegisterAPIs(apiGroup *APIGroup) {
	apisMutex.Lock()
	defer apisMutex.Unlock()

	_, exists := apis[apiGroup.Group]
	if exists {
		logger.Errorf("group %s existed", apiGroup.Group)
	}
	apis[apiGroup.Group] = apiGroup

	logger.Infof("register api group %s", apiGroup.Group)
	apisChangeChan <- struct{}{}
}

func UnregisterAPIs(group string) {
	apisMutex.Lock()
	defer apisMutex.Unlock()

	_, exists := apis[group]
	if !exists {
		logger.Errorf("group %s not found", group)
		return
	}

	delete(apis, group)

	logger.Infof("unregister api group %s", group)
	apisChangeChan <- struct{}{}
}

func (s *Server) registerAPIs() {
	group := &APIGroup{
		Group: "admin",
	}
	group.Entries = append(group.Entries, s.listAPIEntries()...)
	group.Entries = append(group.Entries, s.memberAPIEntries()...)
	group.Entries = append(group.Entries, s.objectAPIEntries()...)
	group.Entries = append(group.Entries, s.metadataAPIEntries()...)
	group.Entries = append(group.Entries, s.healthAPIEntries()...)
	group.Entries = append(group.Entries, s.aboutAPIEntries()...)

	RegisterAPIs(group)
}

func (s *Server) listAPIEntries() []*APIEntry {
	return []*APIEntry{
		{
			Path:    "",
			Method:  "GET",
			Handler: s.listAPIs,
		},
	}
}

func (s *Server) healthAPIEntries() []*APIEntry {
	return []*APIEntry{
		{
			// https://stackoverflow.com/a/43381061/1705845
			Path:    "/healthz",
			Method:  "GET",
			Handler: func(w http.ResponseWriter, r *http.Request) { /* 200 by default */ },
		},
	}
}

func (s *Server) aboutAPIEntries() []*APIEntry {
	return []*APIEntry{
		{
			Path:   "/about",
			Method: "GET",
			Handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				w.Write([]byte(aboutText()))
			},
		},
	}
}

func (s *Server) listAPIs(w http.ResponseWriter, r *http.Request) {
	apisMutex.Lock()
	defer apisMutex.Unlock()

	apiGroups := []*APIGroup{}

	for _, group := range apis {
		apiGroups = append(apiGroups, group)
	}

	sort.Sort(apisbyOrder(apiGroups))

	buff, err := yaml.Marshal(apiGroups)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to yaml failed: %v", apiGroups, err))
	}
	w.Header().Set("Content-Type", "text/vnd.yaml")
	w.Write(buff)
}
