/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package server

import (
	"fmt"

	"github.com/alipay/sofa-mosn/pkg/api/v2"
	"github.com/alipay/sofa-mosn/pkg/log"
	"github.com/alipay/sofa-mosn/pkg/types"
)

var listenerAdapterInstance *ListenerAdapter

type ListenerAdapter struct {
	connHandlerMap     map[string]types.ConnectionHandler // key is server's name
	defaultConnHandler types.ConnectionHandler
	defaultName        string
}

// todo consider to use singleton
func initListenerAdapterInstance(name string, connHandler types.ConnectionHandler) {
	if listenerAdapterInstance == nil {
		listenerAdapterInstance = &ListenerAdapter{
			connHandlerMap: make(map[string]types.ConnectionHandler),
			// we set the first handler as the default handler
			// the handler name should be keeped, so if the handler changed, the default handler changed too.
			defaultName: name,
		}
	}

	// if the handler's name is same as default, the default handler changed too
	if name == listenerAdapterInstance.defaultName {
		listenerAdapterInstance.defaultConnHandler = connHandler
	}

	listenerAdapterInstance.connHandlerMap[name] = connHandler
	log.DefaultLogger.Debugf("add server conn handler, server name = %s", name)
}

func GetListenerAdapterInstance() *ListenerAdapter {
	return listenerAdapterInstance
}

// ResetAdapter only used in test/debug mode
func ResetAdapter() {
	log.DefaultLogger.Infof("adapter reset, only expected in test/debug mode")
	listenerAdapterInstance = nil
}

// AddOrUpdateListener used to:
// Add and start listener when listener doesn't exist
// Update listener when listener already exist
func (adapter *ListenerAdapter) AddOrUpdateListener(serverName string, lc *v2.Listener,
	networkFiltersFactories []types.NetworkFilterChainFactory, streamFiltersFactories []types.StreamFilterChainFactory) error {

	var connHandler types.ConnectionHandler

	if serverName == "" {
		connHandler = adapter.defaultConnHandler
	} else {
		if ch, ok := adapter.connHandlerMap[serverName]; ok {
			connHandler = ch
		} else {
			return fmt.Errorf("AddOrUpdateListener error, servername = %s not found", serverName)
		}
	}

	if connHandler == nil {
		return fmt.Errorf("AddOrUpdateListener called error, connHandler is nil")
	}

	listener, err := connHandler.AddOrUpdateListener(lc, networkFiltersFactories, streamFiltersFactories)

	if err != nil {
		return fmt.Errorf("connHandler.AddOrUpdateListener called error: %s", err.Error())
	}

	if listener == nil {
		return nil
	}

	if al, ok := listener.(*activeListener); ok {
		if !al.updatedLabel {
			// start listener if this is new
			go al.listener.Start(nil)
		}

		return nil
	}

	return fmt.Errorf("AddOrUpdateListener Error, got listener is not activeListener")
}

func (adapter *ListenerAdapter) DeleteListener(serverName string, listenerName string) error {
	var connHandler types.ConnectionHandler

	if serverName == "" {
		connHandler = adapter.defaultConnHandler
	} else {
		if ch, ok := adapter.connHandlerMap[serverName]; ok {
			connHandler = ch
		} else {
			return fmt.Errorf("AddOrUpdateListener error, servername = %s not found", serverName)
		}
	}

	// stop listener first
	if err := connHandler.StopListener(nil, listenerName, true); err != nil {
		return err
	}

	// then remove it from array
	connHandler.RemoveListeners(listenerName)
	return nil
}

func (adapter *ListenerAdapter) UpdateListenerTLS(serverName string, listenerName string, inspector bool, tlsConfigs []v2.TLSConfig) error {
	var connHandler types.ConnectionHandler
	if serverName == "" {
		connHandler = adapter.defaultConnHandler
	} else {
		if ch, ok := adapter.connHandlerMap[serverName]; ok {
			connHandler = ch
		} else {
			return fmt.Errorf("AddOrUpdateListener error, servername = %s not found", serverName)
		}
	}

	if ln := connHandler.FindListenerByName(listenerName); ln != nil {
		cfg := *ln.Config() // should clone a config
		cfg.Inspector = inspector
		cfg.FilterChains = []v2.FilterChain{
			{
				FilterChainConfig: v2.FilterChainConfig{
					FilterChainMatch: cfg.FilterChains[0].FilterChainMatch,
					Filters:          cfg.FilterChains[0].Filters,
					TLSConfigs:       tlsConfigs,
				},
				TLSContexts: tlsConfigs,
			},
		}
		if _, err := connHandler.AddOrUpdateListener(&cfg, nil, nil); err != nil {
			return fmt.Errorf("connHandler.UpdateListenerTLS called error, server:%s, error: %s", serverName, err.Error())
		}
		return nil
	}
	return fmt.Errorf("listener %s is not found", listenerName)

}
