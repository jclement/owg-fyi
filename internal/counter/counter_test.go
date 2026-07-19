package counter

import "testing"

func TestBumpAndPersist(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)
	if n := c.Bump("/about"); n != 1 {
		t.Fatalf("first bump = %d", n)
	}
	if n := c.Bump("/about"); n != 2 {
		t.Fatalf("second bump = %d", n)
	}
	c.Bump("/")
	c.Flush()

	c2 := New(dir)
	if n := c2.Bump("/about"); n != 3 {
		t.Fatalf("after reload bump = %d, want 3", n)
	}
	if n := c2.Bump("/"); n != 2 {
		t.Fatalf("root after reload = %d, want 2", n)
	}
}
