// Code generated by go-swagger; DO NOT EDIT.

package models

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"context"

	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
)

// DeviceProperty device property
//
// swagger:model device_property
type DeviceProperty struct {

	// access mode of the property, Read or ReadWrite
	PropertyAccessMode string `json:"property_access_mode,omitempty"`

	// Description of the property
	PropertyDescription string `json:"property_description,omitempty"`

	// unique identifier of the property
	PropertyIdentifier string `json:"property_identifier,omitempty"`

	// Last activity of the property
	PropertyLastSeen string `json:"property_last_seen,omitempty"`

	// Friendly name of the BLE characteristic.
	PropertyName string `json:"property_name,omitempty"`

	// value of property supporting Read only
	PropertyReading string `json:"property_reading,omitempty"`

	// property service uuid.
	PropertyServiceUUID string `json:"property_service_uuid,omitempty"`

	// value of property allowing ReadWrite
	PropertyState string `json:"property_state,omitempty"`

	// Unit of the property
	PropertyUnit string `json:"property_unit,omitempty"`

	// use device unique identifier to pair property to a device
	WirelessDeviceIdentifier string `json:"wireless_device_identifier,omitempty"`
}

// Validate validates this device property
func (m *DeviceProperty) Validate(formats strfmt.Registry) error {
	return nil
}

// ContextValidate validates this device property based on context it is used
func (m *DeviceProperty) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	return nil
}

// MarshalBinary interface implementation
func (m *DeviceProperty) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *DeviceProperty) UnmarshalBinary(b []byte) error {
	var res DeviceProperty
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}
