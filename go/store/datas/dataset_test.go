// Copyright 2016 Attic Labs, Inc. All rights reserved.
// Licensed under the Apache License, version 2.0:
// http://www.apache.org/licenses/LICENSE-2.0

package datas

import (
	"context"
	"testing"

	"github.com/liquidata-inc/ld/dolt/go/store/chunks"
	"github.com/liquidata-inc/ld/dolt/go/store/types"
	"github.com/stretchr/testify/assert"
)

func TestExplicitBranchUsingDatasets(t *testing.T) {
	assert := assert.New(t)
	id1 := "testdataset"
	id2 := "othertestdataset"
	stg := &chunks.MemoryStorage{}
	store := NewDatabase(stg.NewView())
	defer store.Close()

	ds1 := store.GetDataset(context.Background(), id1)

	// ds1: |a|
	a := types.String("a")
	ds1, err := store.CommitValue(context.Background(), ds1, a)
	assert.NoError(err)
	assert.True(ds1.Head().Get(ValueField).Equals(types.Format_7_18, a))

	// ds1: |a|
	//        \ds2
	ds2 := store.GetDataset(context.Background(), id2)
	ds2, err = store.Commit(context.Background(), ds2, ds1.HeadValue(), CommitOptions{Parents: types.NewSet(context.Background(), store, ds1.HeadRef())})
	assert.NoError(err)
	assert.True(ds2.Head().Get(ValueField).Equals(types.Format_7_18, a))

	// ds1: |a| <- |b|
	b := types.String("b")
	ds1, err = store.CommitValue(context.Background(), ds1, b)
	assert.NoError(err)
	assert.True(ds1.Head().Get(ValueField).Equals(types.Format_7_18, b))

	// ds1: |a|    <- |b|
	//        \ds2 <- |c|
	c := types.String("c")
	ds2, err = store.CommitValue(context.Background(), ds2, c)
	assert.NoError(err)
	assert.True(ds2.Head().Get(ValueField).Equals(types.Format_7_18, c))

	// ds1: |a|    <- |b| <--|d|
	//        \ds2 <- |c| <--/
	mergeParents := types.NewSet(context.Background(), store, types.NewRef(ds1.Head(), types.Format_7_18), types.NewRef(ds2.Head(), types.Format_7_18))
	d := types.String("d")
	ds2, err = store.Commit(context.Background(), ds2, d, CommitOptions{Parents: mergeParents})
	assert.NoError(err)
	assert.True(ds2.Head().Get(ValueField).Equals(types.Format_7_18, d))

	ds1, err = store.Commit(context.Background(), ds1, d, CommitOptions{Parents: mergeParents})
	assert.NoError(err)
	assert.True(ds1.Head().Get(ValueField).Equals(types.Format_7_18, d))
}

func TestTwoClientsWithEmptyDataset(t *testing.T) {
	assert := assert.New(t)
	id1 := "testdataset"
	stg := &chunks.MemoryStorage{}
	store := NewDatabase(stg.NewView())
	defer store.Close()

	dsx := store.GetDataset(context.Background(), id1)
	dsy := store.GetDataset(context.Background(), id1)

	// dsx: || -> |a|
	a := types.String("a")
	dsx, err := store.CommitValue(context.Background(), dsx, a)
	assert.NoError(err)
	assert.True(dsx.Head().Get(ValueField).Equals(types.Format_7_18, a))

	// dsy: || -> |b|
	_, ok := dsy.MaybeHead()
	assert.False(ok)
	b := types.String("b")
	dsy, err = store.CommitValue(context.Background(), dsy, b)
	assert.Error(err)
	// Commit failed, but dsy now has latest head, so we should be able to just try again.
	// dsy: |a| -> |b|
	dsy, err = store.CommitValue(context.Background(), dsy, b)
	assert.NoError(err)
	assert.True(dsy.Head().Get(ValueField).Equals(types.Format_7_18, b))
}

func TestTwoClientsWithNonEmptyDataset(t *testing.T) {
	assert := assert.New(t)
	id1 := "testdataset"
	stg := &chunks.MemoryStorage{}
	store := NewDatabase(stg.NewView())
	defer store.Close()

	a := types.String("a")
	{
		// ds1: || -> |a|
		ds1 := store.GetDataset(context.Background(), id1)
		ds1, err := store.CommitValue(context.Background(), ds1, a)
		assert.NoError(err)
		assert.True(ds1.Head().Get(ValueField).Equals(types.Format_7_18, a))
	}

	dsx := store.GetDataset(context.Background(), id1)
	dsy := store.GetDataset(context.Background(), id1)

	// dsx: |a| -> |b|
	assert.True(dsx.Head().Get(ValueField).Equals(types.Format_7_18, a))
	b := types.String("b")
	dsx, err := store.CommitValue(context.Background(), dsx, b)
	assert.NoError(err)
	assert.True(dsx.Head().Get(ValueField).Equals(types.Format_7_18, b))

	// dsy: |a| -> |c|
	assert.True(dsy.Head().Get(ValueField).Equals(types.Format_7_18, a))
	c := types.String("c")
	dsy, err = store.CommitValue(context.Background(), dsy, c)
	assert.Error(err)
	assert.True(dsy.Head().Get(ValueField).Equals(types.Format_7_18, b))
	// Commit failed, but dsy now has latest head, so we should be able to just try again.
	// dsy: |b| -> |c|
	dsy, err = store.CommitValue(context.Background(), dsy, c)
	assert.NoError(err)
	assert.True(dsy.Head().Get(ValueField).Equals(types.Format_7_18, c))
}

func TestIdValidation(t *testing.T) {
	assert := assert.New(t)
	stg := &chunks.MemoryStorage{}
	store := NewDatabase(stg.NewView())

	invalidDatasetNames := []string{" ", "", "a ", " a", "$", "#", ":", "\n", "💩"}
	for _, id := range invalidDatasetNames {
		assert.Panics(func() {
			store.GetDataset(context.Background(), id)
		})
	}
}

func TestHeadValueFunctions(t *testing.T) {
	assert := assert.New(t)

	id1 := "testdataset"
	id2 := "otherdataset"
	stg := &chunks.MemoryStorage{}
	store := NewDatabase(stg.NewView())
	defer store.Close()

	ds1 := store.GetDataset(context.Background(), id1)
	assert.False(ds1.HasHead())

	// ds1: |a|
	a := types.String("a")
	ds1, err := store.CommitValue(context.Background(), ds1, a)
	assert.NoError(err)
	assert.True(ds1.HasHead())

	hv := ds1.Head().Get(ValueField)
	assert.Equal(a, hv)
	assert.Equal(a, ds1.HeadValue())

	hv, ok := ds1.MaybeHeadValue()
	assert.True(ok)
	assert.Equal(a, hv)

	ds2 := store.GetDataset(context.Background(), id2)
	assert.Panics(func() {
		ds2.HeadValue()
	})
	_, ok = ds2.MaybeHeadValue()
	assert.False(ok)
}

func TestIsValidDatasetName(t *testing.T) {
	assert := assert.New(t)
	cases := []struct {
		name  string
		valid bool
	}{
		{"foo", true},
		{"foo/bar", true},
		{"f1", true},
		{"1f", true},
		{"", false},
		{"f!!", false},
	}
	for _, c := range cases {
		assert.Equal(c.valid, IsValidDatasetName(c.name),
			"Expected %s validity to be %t", c.name, c.valid)
	}
}
