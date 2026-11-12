package msbuild

import (
	"testing"
)

func FuzzParse(f *testing.F) {
	f.Add(`<Project><PropertyGroup><A>1</A></PropertyGroup></Project>`)
	f.Add(`<?xml version="1.0"?><Project Sdk="Microsoft.NET.Sdk"></Project>`)
	f.Add(`<Project xmlns="http://schemas.microsoft.com/developer/msbuild/2003">
  <ItemGroup>
    <Reference Include="A" />
  </ItemGroup>
</Project>
`)
	f.Fuzz(func(t *testing.T, text string) {
		if len(text) > 1<<16 {
			t.Skip()
		}
		els, err := Parse(text)
		if err != nil {
			return
		}
		if len(els) > len(text)+1 {
			t.Fatalf("elems=%d len=%d", len(els), len(text))
		}
	})
}
