/*
Copyright The casbind Authors.
@Date: 2021/03/12 20:05
*/

package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	defaultrolemanager "github.com/casbin/casbin/v2/rbac/default-role-manager"

	model2 "github.com/casbin/casbin/v2/model"

	"github.com/casbin/casbin/v2"
)

// pathExists returns true if the given path exists.
func pathExists(p string) bool {
	if _, err := os.Lstat(p); err != nil && os.IsNotExist(err) {
		return false
	}
	return true
}

// logSize returns the size of the Raft log on disk.
func (s *Store) logSize() (int64, error) {
	fi, err := os.Stat(filepath.Join(s.raftDir, raftDBPath))
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

// dirSize returns the total size of all files in the given directory
func dirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}

// prettyVoter converts bool to "voter" or "non-voter"
func prettyVoter(v bool) string {
	if v {
		return "voter"
	}
	return "non-voter"
}

type EnforcerState struct {
	Model ModelState
}
type ModelState map[string]AssertionStateMap
type AssertionStateMap map[string]AssertionState

type AssertionState struct {
	Key       string
	Value     string
	Tokens    []string
	Policy    [][]string
	PolicyMap map[string]int
}

// CreateEnforcerState transform enforce state to persisting state
func CreateEnforcerState(e *casbin.DistributedEnforcer) (EnforcerState, error) {
	if e == nil {
		return EnforcerState{}, errors.New("nil input")
	}
	m := e.GetModel()
	es := EnforcerState{}
	ms := make(ModelState)

	for k, assertionMap := range m {
		asm := make(AssertionStateMap)
		for k2, v := range assertionMap {
			as := AssertionState{
				Key:       v.Key,
				Value:     v.Value,
				Tokens:    v.Tokens,
				Policy:    v.Policy,
				PolicyMap: v.PolicyMap,
			}
			asm[k2] = as
		}
		ms[k] = asm
	}
	es.Model = ms
	return es, nil
}

// CreateModelFormEnforcerState create enforcer state and add links of rule groups
func CreateModelFormEnforcerState(state EnforcerState) (model2.Model, error) {
	m := model2.NewModel()
	for k, assertionMap := range state.Model {
		am := make(model2.AssertionMap)
		for k2, v := range assertionMap {
			model := model2.Assertion{
				Key:       v.Key,
				Value:     v.Value,
				Tokens:    v.Tokens,
				Policy:    v.Policy,
				PolicyMap: v.PolicyMap,
			}
			if k2 == "g" {
				// link RBAC group polices
				count := strings.Count(model.Value, "_")
				model.RM = defaultrolemanager.NewRoleManager(10)
				for _, rule := range model.Policy {
					if len(rule) < count {
						return m, errors.New("grouping policy elements do not meet role definition")
					}
					if len(rule) > count {
						rule = rule[:count]
					}
					err := model.RM.AddLink(rule[0], rule[1], rule[2:]...)
					if err != nil {
						return m, err
					}
				}
			}
			am[k2] = &model
		}
		m[k] = am
	}

	return m, nil
}
