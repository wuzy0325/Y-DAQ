package driver

import "testing"

// TestPendingResponses_PushIfEmpty 验证"判空 + 入队"原子语义：
// 首个入队返回 true，后续入队返回 false，出队清空后再次返回 true。
// 该返回值决定调用方是否清空响应缓冲区，必须只由首个在途命令获得。
func TestPendingResponses_PushIfEmpty(t *testing.T) {
	q := NewPendingResponses()

	if !q.PushIfEmpty(&PendingEntry{Cmd: "a"}) {
		t.Fatal("空队列首次 PushIfEmpty 应返回 true")
	}
	if q.PushIfEmpty(&PendingEntry{Cmd: "b"}) {
		t.Fatal("非空队列 PushIfEmpty 应返回 false")
	}
	if q.IsEmpty() {
		t.Fatal("入队 2 条后队列不应为空")
	}

	if e := q.Pop(); e == nil || e.Cmd != "a" {
		t.Fatalf("Pop 应返回首个 entry a，实际 %v", e)
	}
	if q.PushIfEmpty(&PendingEntry{Cmd: "c"}) {
		t.Fatal("仍有 1 条在途 entry(b) 时 PushIfEmpty 应返回 false")
	}
	if e := q.Pop(); e == nil || e.Cmd != "b" {
		t.Fatalf("Pop 应返回 entry b，实际 %v", e)
	}
	if e := q.Pop(); e == nil || e.Cmd != "c" {
		t.Fatalf("Pop 应返回 entry c，实际 %v", e)
	}
	if !q.PushIfEmpty(&PendingEntry{Cmd: "d"}) {
		t.Fatal("队列清空后 PushIfEmpty 应再次返回 true")
	}
}
