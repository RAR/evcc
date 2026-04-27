package core

import (
	"testing"

	evbus "github.com/asaskevich/EventBus"
	"github.com/benbjohnson/clock"
	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/core/settings"
	"github.com/evcc-io/evcc/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// vehicleWithCurrent embeds a MockVehicle and additionally implements
// api.CurrentController so api.Cap[api.CurrentController] succeeds via the
// fast-path type assertion.
type vehicleWithCurrent struct {
	*api.MockVehicle
	called  bool
	current int64
}

func (v *vehicleWithCurrent) MaxCurrent(current int64) error {
	v.called = true
	v.current = current
	return nil
}

func TestVehicleMaxCurrentRoutesToVehicle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	charger := api.NewMockCharger(ctrl)
	mockV := api.NewMockVehicle(ctrl)
	mockV.EXPECT().OnIdentified().Return(api.ActionConfig{}).AnyTimes()
	v := &vehicleWithCurrent{MockVehicle: mockV}

	lp := &Loadpoint{
		log:               util.NewLogger("foo"),
		bus:               evbus.New(),
		clock:             clock.NewMock(),
		settings:          settings.NewDatabaseSettingsAdapter("foo"),
		charger:           charger,
		chargeMeter:       &Null{},
		chargeRater:       &Null{},
		chargeTimer:       &Null{},
		wakeUpTimer:       NewTimer(),
		minCurrent:        minA,
		maxCurrent:        maxA,
		offeredCurrent:    minA,
		status:            api.StatusC,
		enabled:           true,
		VehicleMaxCurrent: true,
		vehicle:           v,
	}

	// charger must NOT receive MaxCurrent — vehicle owns it.
	require.NoError(t, lp.setLimit(maxA))
	assert.True(t, v.called, "vehicle MaxCurrent should be called")
	assert.Equal(t, int64(maxA), v.current)
	assert.Equal(t, maxA, lp.offeredCurrent, "offered current must be advanced")
}

func TestVehicleMaxCurrentFallsBackWithoutController(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	charger := api.NewMockCharger(ctrl)
	// vehicle without CurrentController — bare MockVehicle
	v := api.NewMockVehicle(ctrl)
	v.EXPECT().OnIdentified().Return(api.ActionConfig{}).AnyTimes()

	lp := &Loadpoint{
		log:               util.NewLogger("foo"),
		bus:               evbus.New(),
		clock:             clock.NewMock(),
		settings:          settings.NewDatabaseSettingsAdapter("foo"),
		charger:           charger,
		chargeMeter:       &Null{},
		chargeRater:       &Null{},
		chargeTimer:       &Null{},
		wakeUpTimer:       NewTimer(),
		minCurrent:        minA,
		maxCurrent:        maxA,
		offeredCurrent:    minA,
		status:            api.StatusC,
		enabled:           true,
		VehicleMaxCurrent: true,
		vehicle:           v,
	}

	// no CurrentController on vehicle → must fall back to charger
	charger.EXPECT().MaxCurrent(int64(maxA)).Return(nil)
	require.NoError(t, lp.setLimit(maxA))
}

func TestVehicleMaxCurrentDisabledUsesCharger(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	charger := api.NewMockCharger(ctrl)
	mockV := api.NewMockVehicle(ctrl)
	mockV.EXPECT().OnIdentified().Return(api.ActionConfig{}).AnyTimes()
	v := &vehicleWithCurrent{MockVehicle: mockV}

	lp := &Loadpoint{
		log:               util.NewLogger("foo"),
		bus:               evbus.New(),
		clock:             clock.NewMock(),
		settings:          settings.NewDatabaseSettingsAdapter("foo"),
		charger:           charger,
		chargeMeter:       &Null{},
		chargeRater:       &Null{},
		chargeTimer:       &Null{},
		wakeUpTimer:       NewTimer(),
		minCurrent:        minA,
		maxCurrent:        maxA,
		offeredCurrent:    minA,
		status:            api.StatusC,
		enabled:           true,
		VehicleMaxCurrent: false, // explicit
		vehicle:           v,
	}

	// flag off — charger gets the call even though vehicle could accept it
	charger.EXPECT().MaxCurrent(int64(maxA)).Return(nil)
	require.NoError(t, lp.setLimit(maxA))
	assert.False(t, v.called, "vehicle MaxCurrent must NOT be called when flag is off")
}
