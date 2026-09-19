package cmd

import "axtool/internal/state"

func loadState(serverDir string) (state.File, error) {
	return state.Load(serverDir)
}
