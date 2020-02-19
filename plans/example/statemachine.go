package main

import (
	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
	"reflect"
	gort "runtime"
)

func PatternFactory(runenv *runtime.RunEnv, funcs ...func(runenv *runtime.RunEnv) error) *sync.BarrierAllStateMachineNode {
	root := sync.NewBarrierAllStateMachineNode("outer", "setup", runenv)
	root.OnEnter(func() error {
		return SetupStage(runenv)
	})
	current := root
	for _, f := range funcs {
		fname := gort.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
		runenv.RecordMessage("Adding %s to the success path.", fname)
		newnode := sync.NewBarrierAllStateMachineNode("outer", fname, runenv)
		fcpy := f
		closure := sync.StateMachineExecutable(func() error {
			return fcpy(runenv)
		})
		newnode.OnEnter(closure)
		current.AttachSuccess(newnode)
		current = newnode
	}
	end := sync.NewBarrierAllStateMachineNode("outer", "end", runenv)
	end.OnEnter(func() error {
		return TeardownStage(runenv)
	})
	current.AttachSuccess(end)
	return root
}

func SetupStage(runenv *runtime.RunEnv) error {
	runenv.RecordMessage("Setting up")
	return nil
}

func TeardownStage(runenv *runtime.RunEnv) error {
	runenv.RecordMessage("Tearing Down")
	return nil
}

func Stage1(runenv *runtime.RunEnv) error {
	runenv.RecordMessage("Stage 1")
	return nil
}

func Stage2(runenv *runtime.RunEnv) error {
	runenv.RecordMessage("Stage 2")
	return nil
}

func Stage3(runenv *runtime.RunEnv) error {
	runenv.RecordMessage("Stage 3")
	return nil
}

func ExampleStateMachine(runenv *runtime.RunEnv) error {
	runenv.RecordMessage("creating state machine with stage 1, 2, and 3")
	sm := PatternFactory(runenv, Stage1, Stage2, Stage3)
	_ = sm.Enter()
	return nil
}
