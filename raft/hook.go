package raft

// Hook will be executed each time Raft node receives a new value.
type OnNewValue func(Item) error

// NilHook defines a null behaviour for OnNewVaue
func NilHook(Item) error {
	return nil
}
