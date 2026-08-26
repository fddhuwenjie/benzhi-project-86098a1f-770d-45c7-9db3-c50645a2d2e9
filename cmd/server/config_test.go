package main

import "testing"

func TestPortEnvironmentAndAddressValidation(t *testing.T) {
	configuration, err := parseConfig(nil, func(name string) string {
		if name == "PORT" {
			return "19123"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if configuration.Address != "127.0.0.1:19123" {
		t.Fatalf("PORT 未形成回环监听地址: %s", configuration.Address)
	}
	if _, err := parseConfig([]string{"-addr=0.0.0.0:19081"}, func(string) string { return "" }); err == nil {
		t.Fatal("不应允许非回环监听地址")
	}
}
