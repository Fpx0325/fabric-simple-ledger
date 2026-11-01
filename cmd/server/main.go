package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/example/fabric-simple-ledger/internal/ledger"
	_ "github.com/go-sql-driver/mysql"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"
)

type Chain interface {
	Create(context.Context, string, string, string) ([]byte, error)
	Read(context.Context, string) ([]byte, error)
}
type app struct {
	db    *sql.DB
	chain Chain
	token string
}

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, msg string) {
	reply(w, status, map[string]string{"error": msg})
}
func (a *app) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+a.token)) != 1 {
			fail(w, 401, "访问令牌无效")
			return
		}
		next(w, r)
	}
}
func (a *app) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		fail(w, 400, "JSON 无效或内容超过 64KB")
		return
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		fail(w, 400, "只允许一个 JSON 对象")
		return
	}
	if !idPattern.MatchString(in.ID) || len(strings.TrimSpace(in.Title)) == 0 || len(in.Title) > 240 || len(in.Content) == 0 {
		fail(w, 400, "编号须为 1–64 位字母、数字、下划线或连字符；标题不超过 240 字节；内容不能为空")
		return
	}
	sum := sha256.Sum256([]byte(in.Content))
	hash := hex.EncodeToString(sum[:])
	// Persist intent BEFORE submitting. Uncertain outcomes are never called failures.
	result, err := a.db.ExecContext(r.Context(), "INSERT INTO operations(evidence_id,action,status) VALUES(?, 'CREATE','PENDING')", in.ID)
	if err != nil {
		fail(w, 503, "日志数据库不可用，未提交交易")
		return
	}
	op, err := result.LastInsertId()
	if err != nil {
		fail(w, 503, "无法取得操作编号，未提交交易")
		return
	}
	data, chainErr := a.chain.Create(r.Context(), in.ID, in.Title, hash)
	status := "COMMITTED"
	tx := ""
	if chainErr != nil {
		status = "UNKNOWN"
	} else {
		var e struct {
			TxID string `json:"tx_id"`
		}
		json.Unmarshal(data, &e)
		tx = e.TxID
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = a.db.ExecContext(ctx, "UPDATE operations SET status=?,tx_id=? WHERE id=?", status, tx, op)
	if chainErr != nil {
		slog.Warn("submission outcome uncertain", "operation", op, "error", chainErr)
		reply(w, 502, map[string]any{"error": "交易未确认，请先按编号查询链上结果，勿盲目重试", "operation_id": op, "status": "UNKNOWN"})
		return
	}
	if err != nil {
		slog.Error("audit finalization failed", "operation", op)
		reply(w, 202, map[string]any{"evidence": json.RawMessage(data), "operation_id": op, "warning": "链上已确认；日志仍为 PENDING，请执行 reconcile"})
		return
	}
	reply(w, 201, map[string]any{"evidence": json.RawMessage(data), "operation_id": op})
}
func (a *app) read(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !idPattern.MatchString(id) {
		fail(w, 400, "编号格式无效")
		return
	}
	data, err := a.chain.Read(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "NOT_FOUND:") {
			fail(w, 404, "未找到该存证")
		} else {
			fail(w, 502, "链上查询失败，请检查 Fabric 网络")
		}
		return
	}
	_, err = a.db.ExecContext(r.Context(), "INSERT INTO operations(evidence_id,action,status) VALUES(?,'READ','SUCCESS')", id)
	if err != nil {
		reply(w, 202, map[string]any{"evidence": json.RawMessage(data), "warning": "查询成功，但查询日志写入失败"})
		return
	}
	reply(w, 200, map[string]any{"evidence": json.RawMessage(data)})
}
func (a *app) logs(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.QueryContext(r.Context(), "SELECT id,evidence_id,action,status,tx_id,created_at FROM operations ORDER BY id DESC LIMIT 50")
	if err != nil {
		fail(w, 503, "日志查询失败")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int64
		var eid, action, status, tx, created string
		if err = rows.Scan(&id, &eid, &action, &status, &tx, &created); err != nil {
			fail(w, 503, "日志读取失败")
			return
		}
		items = append(items, map[string]any{"id": id, "evidence_id": eid, "action": action, "status": status, "tx_id": tx, "created_at": created})
	}
	if rows.Err() != nil {
		fail(w, 503, "日志读取失败")
		return
	}
	reply(w, 200, items)
}
func (a *app) reconcile(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.QueryContext(r.Context(), "SELECT id,evidence_id FROM operations WHERE action='CREATE' AND status IN ('PENDING','UNKNOWN') LIMIT 100")
	if err != nil {
		fail(w, 503, "数据库不可用")
		return
	}
	type pending struct {
		id  int64
		eid string
	}
	var list []pending
	for rows.Next() {
		var p pending
		if err = rows.Scan(&p.id, &p.eid); err != nil {
			break
		}
		list = append(list, p)
	}
	rowErr := rows.Err()
	rows.Close()
	if err != nil || rowErr != nil {
		fail(w, 503, "读取待核对日志失败")
		return
	}
	n := 0
	for _, p := range list {
		data, e := a.chain.Read(r.Context(), p.eid)
		if e != nil {
			continue
		}
		var v struct {
			Tx string `json:"tx_id"`
		}
		if json.Unmarshal(data, &v) != nil {
			continue
		} // A matching ID proves existence, not that this particular request committed.
		if _, e = a.db.ExecContext(r.Context(), "UPDATE operations SET status='FOUND_ON_CHAIN',tx_id=? WHERE id=? AND status IN ('PENDING','UNKNOWN')", v.Tx, p.id); e != nil {
			fail(w, 503, "部分核对成功，数据库更新失败")
			return
		}
		n++
	}
	reply(w, 200, map[string]int{"reconciled": n})
}
func (a *app) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/evidence", a.auth(a.create))
	mux.HandleFunc("GET /api/evidence/{id}", a.auth(a.read))
	mux.HandleFunc("GET /api/operations", a.auth(a.logs))
	mux.HandleFunc("POST /api/reconcile", a.auth(a.reconcile))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { reply(w, 200, map[string]string{"status": "alive"}) })
	mux.Handle("GET /", http.FileServer(http.Dir("web")))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		mux.ServeHTTP(w, r)
	})
}
func main() {
	token := os.Getenv("API_TOKEN")
	if len(token) < 32 {
		slog.Error("API_TOKEN must contain at least 32 characters")
		os.Exit(1)
	}
	db, err := sql.Open("mysql", os.Getenv("MYSQL_DSN"))
	if err != nil {
		panic(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(3 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = db.PingContext(ctx); err != nil {
		panic(err)
	}
	gw, err := ledger.Open()
	if err != nil {
		panic(err)
	}
	defer gw.Close()
	a := &app{db, gw, token}
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	srv := &http.Server{Addr: addr, Handler: a.routes(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second}
	stop, done := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer done()
	go func() {
		<-stop.Done()
		ctx, c := context.WithTimeout(context.Background(), 10*time.Second)
		defer c()
		srv.Shutdown(ctx)
	}()
	slog.Info("listening", "address", addr)
	if err = srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}
