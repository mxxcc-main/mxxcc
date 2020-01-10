// Copyright 2019 The go-ccmchain Authors
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

package ccm

import (
	"github.com/ccmchain/go-ccmchain/core"
	"github.com/ccmchain/go-ccmchain/core/forkid"
	"github.com/ccmchain/go-ccmchain/p2p/enode"
	"github.com/ccmchain/go-ccmchain/rlp"
)

// ccmEntry is the "ccm" ENR entry which advertises ccm protocol
// on the discovery network.
type ccmEntry struct {
	ForkID forkid.ID // Fork identifier per EIP-2124

	// Ignore additional fields (for forward compatibility).
	Rest []rlp.RawValue `rlp:"tail"`
}

// ENRKey implements enr.Entry.
func (e ccmEntry) ENRKey() string {
	return "ccm"
}

func (ccm *Ccmchain) startEthEntryUpdate(ln *enode.LocalNode) {
	var newHead = make(chan core.ChainHeadEvent, 10)
	sub := ccm.blockchain.SubscribeChainHeadEvent(newHead)

	go func() {
		defer sub.Unsubscribe()
		for {
			select {
			case <-newHead:
				ln.Set(ccm.currentEthEntry())
			case <-sub.Err():
				// Would be nice to sync with ccm.Stop, but there is no
				// good way to do that.
				return
			}
		}
	}()
}

func (ccm *Ccmchain) currentEthEntry() *ccmEntry {
	return &ccmEntry{ForkID: forkid.NewID(ccm.blockchain)}
}
