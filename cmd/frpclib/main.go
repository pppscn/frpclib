// Copyright 2016 fatedier, fatedier@gmail.com
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

package frpclib

import (
	"strings"

	_ "github.com/fatedier/frp/assets/frpc"
	"github.com/fatedier/frp/cmd/frpclib/sub"
	"github.com/fatedier/frp/pkg/util/version"
	"github.com/fatedier/frp/pkg/util/system"
)

func main() {
	system.EnableCompatibilityMode()
	sub.Execute()
}

func GetVersion() string {
	return version.Full()
}

func RunFile(uid string, cfgFilePath string) (errString string) {
	err, _ := sub.RunClientWithUid(uid, cfgFilePath)
	if err != nil {
		return err.Error()
	}
	return ""
}

func RunContent(uid string, cfgContent string) (errString string) {
	err, _ := sub.RunClientByContent(uid, cfgContent)
	if err != nil {
		return err.Error()
	}
	return ""
}

func Close(uid string) (ret bool) {
	return sub.Close(uid)
}

func GetUids() string {
	uids := sub.GetUids()
	return strings.Join(uids, ",")
}

func IsRunning(uid string) (running bool) {
	return sub.IsRunning(uid)
}
