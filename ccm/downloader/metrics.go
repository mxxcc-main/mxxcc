// Copyright 2015 The go-ccmchain Authors
// This file is part of the go-ccmchain library.
//
// The go-ccmchain library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ccmchain library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ccmchain library. If not, see <http://www.gnu.org/licenses/>.

// Contains the metrics collected by the downloader.

package downloader

import (
	"github.com/ccmchain/go-ccmchain/metrics"
)

var (
	headerInMeter      = metrics.NewRegisteredMeter("ccm/downloader/headers/in", nil)
	headerReqTimer     = metrics.NewRegisteredTimer("ccm/downloader/headers/req", nil)
	headerDropMeter    = metrics.NewRegisteredMeter("ccm/downloader/headers/drop", nil)
	headerTimeoutMeter = metrics.NewRegisteredMeter("ccm/downloader/headers/timeout", nil)

	bodyInMeter      = metrics.NewRegisteredMeter("ccm/downloader/bodies/in", nil)
	bodyReqTimer     = metrics.NewRegisteredTimer("ccm/downloader/bodies/req", nil)
	bodyDropMeter    = metrics.NewRegisteredMeter("ccm/downloader/bodies/drop", nil)
	bodyTimeoutMeter = metrics.NewRegisteredMeter("ccm/downloader/bodies/timeout", nil)

	receiptInMeter      = metrics.NewRegisteredMeter("ccm/downloader/receipts/in", nil)
	receiptReqTimer     = metrics.NewRegisteredTimer("ccm/downloader/receipts/req", nil)
	receiptDropMeter    = metrics.NewRegisteredMeter("ccm/downloader/receipts/drop", nil)
	receiptTimeoutMeter = metrics.NewRegisteredMeter("ccm/downloader/receipts/timeout", nil)

	stateInMeter   = metrics.NewRegisteredMeter("ccm/downloader/states/in", nil)
	stateDropMeter = metrics.NewRegisteredMeter("ccm/downloader/states/drop", nil)
)
