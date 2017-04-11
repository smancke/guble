package configstring

import (
	"github.com/cosminrentea/gobbler/testutil"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestList_IsEmpty(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()

	m := NewMockSettings(ctrl)
	m.EXPECT().SetValue(gomock.Any())

	listFromKingpin := NewFromKingpin(m)

	assert.True(t, listFromKingpin.IsEmpty())

	list := &List{}

	assert.True(t, list.IsEmpty())
}
