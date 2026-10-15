package uw

import (
	"os"
	"time"

	"github.com/memmaker/uw1go/internal/mp"
)

// Saved games while online: a save is made on both machines, and each slot keeps the
// other player too (PEER.DAT, their PLAYER.DAT), so either can restore the session for
// both. Saving sends "save" (the slot, its description and the saver's player); the other
// saves to the same slot, keeps the saver as its peer and answers with its own player
// ("saved2"), which the saver keeps as its peer. Restoring such a slot gives the other
// their player back ("player") ahead of the world (sendWorld).

const peerFile = "peer.dat"

var (
	peerSave   []byte // the other player from the slot just restored, to give them
	savingPeer bool   // saving for the other player's "save"
)

// onlineSaved follows a save to slot made here.
func onlineSaved(slot int, desc string) {
	if online == nil || savingPeer || len(peers) == 0 {
		return
	}
	online.Send(mp.Msg{Op: "save", Level: slot, Text: desc, Data: readFile(homeDir + "player.dat")})
}

// takePeerSave keeps the restored slot's other player aside (and out of SAVE0, so that
// later saves made alone do not carry it).
func takePeerSave() {
	peerSave = readFile(homeDir + peerFile)
	os.Remove(dosPath(homeDir + peerFile))
}

// sendPeerSave gives the other player their player from the restored slot.
func sendPeerSave() {
	if peerSave != nil {
		online.Send(mp.Msg{Op: "player", Data: peerSave})
		peerSave = nil
	}
}

func onlineSaveEvent(m mp.Msg) {
	dprintf("mp %s slot %d\n", m.Op, m.Level)
	switch m.Op {
	case "save": // the other saved: save here too, keeping them
		setCreaturesToSavedGame()
		savingPeer = true
		ok := saveToSlot(m.Level, m.Text)
		savingPeer = false
		if ok {
			blttodrive(m.Data, slotDir(m.Level)+peerFile)
			scrollPrint("Saved with the other player.\n")
			online.Send(mp.Msg{Op: "saved2", Level: m.Level, Data: readFile(homeDir + "player.dat")})
		}
	case "saved2": // the other's player for the slot saved here
		blttodrive(m.Data, slotDir(m.Level)+peerFile)
	case "player": // the other restored a session: this player as it was saved
		if len(m.Data) == 0 || !blttodrive(m.Data, homeDir+"player.dat") {
			return
		}
		if GrSq >= 0 {
			ObjRem(&Level.Tiles[GrSq].Objects, ThePlayer)
		}
		GrSq = -1
		if RestorePlayerInv(homeDir) && GetLevel(PlayerLevel) != 0 {
			playerChangeSq()
			syncLevel = PlayerLevel
			resetSync()
			synced = false // the world follows
			wantAt = time.Now()
			restoredScreen()
			scrollPrint("The other player restored a saved game.\n")
		}
	}
}
