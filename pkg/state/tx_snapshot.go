package state

import (
	"github.com/pkg/errors"

	"github.com/wavesplatform/gowaves/pkg/proto"
)

type snapshotApplierHooks interface {
	BeforeTxSnapshotApply() error
	AfterTxSnapshotApply() error
}

type extendedSnapshotApplier interface {
	SetApplierInfo(info *blockSnapshotsApplierInfo)
	proto.SnapshotApplier
	internalSnapshotApplier
	snapshotApplierHooks
}

type txSnapshot struct {
	regular  []proto.AtomicSnapshot
	internal []internalSnapshot
}

func (ts txSnapshot) Apply(a extendedSnapshotApplier) error {
	if err := a.BeforeTxSnapshotApply(); err != nil {
		return errors.Wrapf(err, "failed to execute before tx snapshot apply hook")
	}
	// internal snapshots must be applied at the end
	for _, rs := range ts.regular {
		if !rs.IsGeneratedByTxDiff() {
			err := rs.Apply(a)
			if err != nil {
				return errors.Wrap(err, "failed to apply regular transaction snapshot")
			}
		}
	}
	for _, is := range ts.internal {
		err := is.ApplyInternal(a)
		if err != nil {
			return errors.Wrap(err, "failed to apply internal transaction snapshot")
		}
	}
	if err := a.AfterTxSnapshotApply(); err != nil {
		return errors.Wrapf(err, "failed to execute after tx snapshot apply hook")
	}
	return nil
}