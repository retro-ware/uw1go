package uw

// CONVVARS.C: handing the NPC's and the player's state to a conversation's imported
// variables before it runs, and taking back what it may change afterwards. Only names the
// script imports are touched.

func setupConverseData(npc *Object) {
	crit := &Creatures[npc.InMajor()]
	health := func(hp byte, avg byte) int {
		if avg != 0 {
			return int(int16(int(hp) << 8 / int(avg)))
		}
		return 0x80
	}
	babVar("npc_whoami", int(npc.Whoami))
	if npc.Fed() != 0 {
		babVar("npc_hunger", 0x10)
	} else {
		babVar("npc_hunger", 0xC0)
	}
	babVar("npc_health", health(npc.HP, crit.Avghit))
	babVar("npc_hp", int(npc.HP))
	babVar("npc_arms", int(crit.Attacks[0].Chance))
	babVar("npc_power", int(crit.Attr[ATTR_STR])+crit.Caster())
	babVar("npc_goal", npc.Goal())
	babVar("npc_gtarg", npc.GTarg())
	babVar("npc_talkedto", npc.TalkedTo())
	babVar("npc_level", crit.Level())
	babVar("npc_xhome", npc.Quality())
	babVar("npc_yhome", npc.Owner())
	if who := int(npc.Whoami); who != 0 {
		babVar("npc_name", (who+0x10)|STR_CONV)
	} else {
		babVar("npc_name", npc.Item()|STR_OBJNAMES)
	}
	switch {
	case npc.Goal() == GOAL_ATTACK && npc.GTarg() == 1:
		babVar("npc_attitude", 0)
	case npc.Ally() != 0:
		babVar("npc_attitude", 6)
	default:
		babVar("npc_attitude", npc.Attitude())
	}

	crit = &Creatures[ThePlayer.InMajor()]
	babVar("play_hunger", int(player.Hunger))
	babVar("play_health", health(ThePlayer.HP, crit.Avghit))
	babVar("play_hp", int(ThePlayer.HP))
	babVar("play_arms", int(player.Skills[SKILL_ATTACK])+int(player.Strength))
	babVar("play_power", int(player.Dexterity)+int(player.PlayMana)+int(player.Skills[SKILL_MISSILE]))
	babVar("play_mana", int(player.PlayMana))
	babVar("play_level", int(player.Level))
	babVar("dungeon_level", PlayerLevel)
	babVar("game_time", int(int16(player.GameClock/CONV_MINUTE)))
	babVar("game_mins", int(player.GameClock/CONV_MINUTE%CONV_DAY_MINS))
	babVar("game_days", int(player.GameClock/CONV_DAY))
	babVar("new_player_exp", 0)
	babVar("play_sex", player.Female())
	babVar("play_poison", player.Poison())
	babVar("play_drawn", player.Drawn())
	babVar("play_name", playerNameHandle)
}

// updateConverseData copies back what a conversation may change; true when the script
// left npc_attitude at 0. A variable the script lacks leaves the last value read, as the
// original's one local does.
func updateConverseData(npc *Object) bool {
	var val, gtarg int16
	babVarOut("npc_hunger", &val)
	npc.SetFed(b2i(val < 0x20))
	babVarOut("npc_hp", &val)
	npc.HP = byte(val)
	babVarOut("npc_xhome", &val)
	npc.SetQuality(int(val))
	babVarOut("npc_yhome", &val)
	npc.SetOwner(int(val))
	babVarOut("npc_goal", &val)
	babVarOut("npc_gtarg", &gtarg)
	changeCritterGoal(npc, int(val), int(gtarg))
	npc.SetTalkedTo(1)
	babVarOut("npc_attitude", &val)
	if val > 3 {
		npc.SetAttitude(ATT_FRIENDLY)
		npc.SetAlly(1)
	} else {
		npc.SetAttitude(int(val))
	}
	killed := val == 0
	babVarOut("play_hunger", &val)
	player.Hunger = byte(val)
	babVarOut("play_hp", &val)
	ThePlayer.HP = byte(val)
	babVarOut("play_mana", &val)
	player.PlayMana = byte(val)
	babVarOut("play_poison", &val)
	player.SetPoison(int(val))
	babVarOut("new_player_exp", &val)
	if val != 0 {
		playerGetExp(int(val))
	}
	return killed
}
