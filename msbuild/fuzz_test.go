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
	f.Add(project)
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

func FuzzSetProperty(f *testing.F) {
	f.Add(project, "Version", "2.0")
	f.Add(`<Project><PropertyGroup><A>1</A></PropertyGroup></Project>`, "A", "2")
	f.Add(`<?xml version="1.0"?><Project Sdk="Microsoft.NET.Sdk"></Project>`, "Version", "1")
	f.Fuzz(func(t *testing.T, text, name, value string) {
		if len(text) > 1<<16 {
			t.Skip()
		}
		out, err := SetProperty(text, name, value)
		if err != nil {
			if out != text {
				t.Fatalf("a refusal changed the text")
			}
			return
		}
		if _, err := Parse(out); err != nil {
			t.Fatalf("SetProperty wrote a file that does not parse: %v", err)
		}
	})
}

func FuzzSetAttr(f *testing.F) {
	f.Add(project, "Private", "true")
	f.Add(`<Project><ItemGroup><Reference Include="A" /></ItemGroup></Project>`, "Include", "B")
	f.Fuzz(func(t *testing.T, text, attr, value string) {
		if len(text) > 1<<16 {
			t.Skip()
		}
		out, err := SetAttr(text, func(Elem, string) bool { return true }, attr, value)
		if err != nil {
			if out != text {
				t.Fatalf("a refusal changed the text")
			}
			return
		}
		if _, err := Parse(out); err != nil {
			t.Fatalf("SetAttr wrote a file that does not parse: %v", err)
		}
	})
}
