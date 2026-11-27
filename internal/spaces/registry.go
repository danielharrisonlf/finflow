package spaces

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"finflow/internal/db"
	"finflow/internal/domain"
	"finflow/internal/service"
	"finflow/internal/store"
)

// Mensagens de quando o banco atual não permite espaços adicionais.
const (
	UnsupportedPostgres = "Espaços adicionais não estão disponíveis com PostgreSQL (DATABASE_URL): o FinFlow só cria espaços como bancos SQLite locais. Use o SQLite (FINFLOW_DB_PATH) para ter espaços Casa e Empresa."
	UnsupportedMemory   = "Espaços adicionais não estão disponíveis com o banco em memória."
)

// Options configura o registro de espaços.
type Options struct {
	// DataDir é a pasta do banco pessoal, onde ficam spaces.json e spaces/<slug>.db; vazio = só o
	// espaço pessoal (Unsupported explica o motivo).
	DataDir     string
	Unsupported string
	// PersonalFile é o caminho do banco pessoal (só informativo no catálogo).
	PersonalFile string
	// HTTPClient é repassado aos serviços de cada espaço (nil = padrão).
	HTTPClient *http.Client
}

// OptionsFor deriva as opções do banco pessoal aberto: SQLite em arquivo habilita espaços na mesma
// pasta; PostgreSQL e SQLite em memória ficam só com o espaço pessoal.
func OptionsFor(d *db.DB) Options {
	if d == nil || d.Dialect == db.DialectPostgres {
		return Options{Unsupported: UnsupportedPostgres}
	}
	target := d.Target
	if i := strings.IndexByte(target, '?'); i >= 0 {
		target = target[:i]
	}
	target = strings.TrimPrefix(target, "file:")
	if target == "" || strings.HasPrefix(target, ":memory:") || strings.Contains(d.Target, "mode=memory") {
		return Options{Unsupported: UnsupportedMemory}
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		abs = target
	}
	return Options{DataDir: filepath.Dir(abs), PersonalFile: abs}
}

type openSpace struct {
	db   *db.DB
	svcs *service.Services
}

// Registry abre sob demanda e mantém em cache os serviços de cada espaço. É seguro para uso
// concorrente; o catálogo é relido do disco a cada operação (outro processo, como a CLI, pode
// ter criado um espaço com o servidor rodando).
type Registry struct {
	mu        sync.Mutex
	base      *service.Services
	opts      Options
	open      map[string]*openSpace
	closed    bool
	startedAt string
}

// New cria o registro com o espaço pessoal já aberto (base). O registro não fecha o banco pessoal.
func New(base *service.Services, opts Options) *Registry {
	return &Registry{
		base:      base,
		opts:      opts,
		open:      map[string]*openSpace{},
		startedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// Single é o registro só com o espaço pessoal (testes e modos sem pasta de dados).
func Single(base *service.Services) *Registry {
	return New(base, Options{Unsupported: UnsupportedMemory})
}

// Supported indica se o registro pode criar espaços adicionais.
func (r *Registry) Supported() bool { return r.opts.DataDir != "" }

// Base devolve os serviços do espaço pessoal.
func (r *Registry) Base() *service.Services { return r.base }

func (r *Registry) catalogPath() string { return filepath.Join(r.opts.DataDir, CatalogFile) }

func (r *Registry) spacesDir() string { return filepath.Join(r.opts.DataDir, "spaces") }

// spacePath é sempre derivado do slug (já validado), nunca do campo "file" do catálogo.
func (r *Registry) spacePath(slug string) string {
	return filepath.Join(r.spacesDir(), slug+".db")
}

func (r *Registry) unsupported() error {
	return &service.AppError{Status: http.StatusNotImplemented, Message: r.opts.Unsupported}
}

func notFound() error { return service.NotFound("espaço não encontrado") }

func (r *Registry) load() (catalog, error) {
	if !r.Supported() {
		return catalog{}, nil
	}
	return readCatalog(r.catalogPath())
}

// personalEntry devolve o espaço pessoal: o do catálogo (nome e cor editáveis) ou o padrão.
func (r *Registry) personalEntry(c catalog) catalogEntry {
	e := catalogEntry{
		Slug:  domain.DefaultSpaceSlug,
		Name:  "Pessoal",
		Kind:  domain.SpacePersonal,
		Color: domain.DefaultSpaceColor(domain.SpacePersonal),
	}
	if r.opts.PersonalFile != "" {
		e.File = filepath.Base(r.opts.PersonalFile)
		if st, err := os.Stat(r.opts.PersonalFile); err == nil {
			e.CreatedAt = st.ModTime().UTC().Format(time.RFC3339)
		}
	}
	if e.CreatedAt == "" {
		e.CreatedAt = r.startedAt
	}
	if i := c.find(domain.DefaultSpaceSlug); i >= 0 {
		saved := c.Spaces[i]
		e.Name, e.Color = saved.Name, saved.Color
		if saved.CreatedAt != "" {
			e.CreatedAt = saved.CreatedAt
		}
	}
	e.Kind = domain.SpacePersonal
	return e
}

func toSpace(e catalogEntry) domain.Space {
	return domain.Space{
		Slug:      e.Slug,
		Name:      e.Name,
		Kind:      e.Kind,
		Color:     e.Color,
		CreatedAt: e.CreatedAt,
		IsDefault: e.Slug == domain.DefaultSpaceSlug,
	}
}

// List devolve o espaço pessoal seguido dos demais, na ordem do catálogo.
func (r *Registry) List() ([]domain.Space, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, err := r.load()
	if err != nil {
		return nil, err
	}
	return r.list(c), nil
}

func (r *Registry) list(c catalog) []domain.Space {
	out := []domain.Space{toSpace(r.personalEntry(c))}
	for _, e := range c.Spaces {
		if e.Slug != domain.DefaultSpaceSlug {
			out = append(out, toSpace(e))
		}
	}
	return out
}

// Get devolve um espaço pelo slug (404 se não existir).
func (r *Registry) Get(slug string) (domain.Space, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, err := r.load()
	if err != nil {
		return domain.Space{}, err
	}
	return r.get(c, slug)
}

func (r *Registry) get(c catalog, slug string) (domain.Space, error) {
	if slug == "" || slug == domain.DefaultSpaceSlug {
		return toSpace(r.personalEntry(c)), nil
	}
	if !domain.IsValidSpaceSlug(slug) {
		return domain.Space{}, notFound()
	}
	i := c.find(slug)
	if i < 0 {
		return domain.Space{}, notFound()
	}
	return toSpace(c.Spaces[i]), nil
}

// Services devolve os serviços do espaço (vazio = pessoal), abrindo o banco na primeira vez
// (Migrate + Bootstrap com as categorias padrão do tipo). Slug desconhecido = 404.
func (r *Registry) Services(ctx context.Context, slug string) (*service.Services, error) {
	if slug == "" || slug == domain.DefaultSpaceSlug {
		return r.base, nil
	}
	if !domain.IsValidSpaceSlug(slug) || !r.Supported() {
		return nil, notFound()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil, errors.New("registro de espaços encerrado")
	}
	if sp, ok := r.open[slug]; ok {
		return sp.svcs, nil
	}
	c, err := r.load()
	if err != nil {
		return nil, err
	}
	i := c.find(slug)
	if i < 0 {
		return nil, notFound()
	}
	sp, err := r.openSpace(ctx, slug, c.Spaces[i].Kind)
	if err != nil {
		return nil, err
	}
	r.open[slug] = sp
	return sp.svcs, nil
}

func (r *Registry) openSpace(ctx context.Context, slug string, kind domain.SpaceKind) (*openSpace, error) {
	database, err := db.Open(ctx, "", r.spacePath(slug))
	if err != nil {
		return nil, fmt.Errorf("abrir espaço %q: %w", slug, err)
	}
	st := store.New(database)
	if err := store.BootstrapKind(ctx, st, kind); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("preparar espaço %q: %w", slug, err)
	}
	return &openSpace{db: database, svcs: service.New(st, r.opts.HTTPClient)}, nil
}

// slugTaken considera ocupado o slug reservado, o do catálogo e um arquivo de banco já existente
// (ex.: sobra de uma cópia manual), para nunca reaproveitar dados sem querer.
func (r *Registry) slugTaken(c catalog, slug string) bool {
	if slug == domain.DefaultSpaceSlug || c.find(slug) >= 0 {
		return true
	}
	_, err := os.Stat(r.spacePath(slug))
	return err == nil
}

// Create cria o espaço: banco novo com as categorias padrão do tipo e entrada no catálogo.
func (r *Registry) Create(ctx context.Context, in domain.CreateSpaceInput) (domain.Space, error) {
	if issues := domain.ValidateCreateSpace(in); len(issues) > 0 {
		return domain.Space{}, service.ValidationFailed(issues)
	}
	if !r.Supported() {
		return domain.Space{}, r.unsupported()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return domain.Space{}, errors.New("registro de espaços encerrado")
	}
	c, err := r.load()
	if err != nil {
		return domain.Space{}, err
	}
	var slug string
	if in.Slug != nil {
		slug = *in.Slug
		if r.slugTaken(c, slug) {
			return domain.Space{}, service.Conflict(fmt.Sprintf("já existe um espaço com o slug %q", slug))
		}
	} else {
		base := domain.SlugifySpaceName(in.Name)
		slug = base
		for n := 2; r.slugTaken(c, slug); n++ {
			suffix := fmt.Sprintf("-%d", n)
			slug = strings.TrimRight(base[:min(len(base), 31-len(suffix))], "-") + suffix
		}
	}
	color := domain.DefaultSpaceColor(in.Kind)
	if in.Color != nil {
		color = *in.Color
	}
	entry := catalogEntry{
		Slug:      slug,
		Name:      strings.TrimSpace(in.Name),
		Kind:      in.Kind,
		Color:     color,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		File:      "spaces/" + slug + ".db",
	}
	sp, err := r.openSpace(ctx, slug, in.Kind)
	if err != nil {
		return domain.Space{}, err
	}
	if c.find(domain.DefaultSpaceSlug) < 0 {
		c.Spaces = append([]catalogEntry{r.personalEntry(c)}, c.Spaces...)
	}
	c.Spaces = append(c.Spaces, entry)
	if err := writeCatalog(r.catalogPath(), c); err != nil {
		_ = sp.db.Close()
		removeDBFiles(r.spacePath(slug))
		return domain.Space{}, err
	}
	r.open[slug] = sp
	return toSpace(entry), nil
}

// Update altera nome e cor (o pessoal também, gravado no catálogo).
func (r *Registry) Update(slug string, in domain.UpdateSpaceInput) (domain.Space, error) {
	if issues := domain.ValidateUpdateSpace(in); len(issues) > 0 {
		return domain.Space{}, service.ValidationFailed(issues)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	c, err := r.load()
	if err != nil {
		return domain.Space{}, err
	}
	if _, err := r.get(c, slug); err != nil {
		return domain.Space{}, err
	}
	if !r.Supported() {
		return domain.Space{}, r.unsupported()
	}
	i := c.find(slug)
	if i < 0 { // pessoal ainda fora do catálogo
		c.Spaces = append([]catalogEntry{r.personalEntry(c)}, c.Spaces...)
		i = 0
	}
	if in.Name.Set {
		c.Spaces[i].Name = strings.TrimSpace(in.Name.Value)
	}
	if in.Color.Set {
		c.Spaces[i].Color = in.Color.Value
	}
	if err := writeCatalog(r.catalogPath(), c); err != nil {
		return domain.Space{}, err
	}
	if slug == domain.DefaultSpaceSlug {
		return toSpace(r.personalEntry(c)), nil
	}
	return toSpace(c.Spaces[i]), nil
}

// Delete tira o espaço do catálogo e move o banco para <dataDir>/spaces/trash/<slug>-<data>.db
// (não apaga de fato). O pessoal não pode ser apagado (409); confirm precisa repetir o slug.
func (r *Registry) Delete(slug, confirm string) error {
	if slug == domain.DefaultSpaceSlug {
		return service.Conflict("o espaço pessoal não pode ser apagado")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	c, err := r.load()
	if err != nil {
		return err
	}
	if _, err := r.get(c, slug); err != nil {
		return err
	}
	if confirm != slug {
		return service.BadRequest("confirme a exclusão repetindo o slug do espaço em ?confirm=")
	}
	// No Windows um arquivo aberto não pode ser movido: fecha o banco antes.
	if sp, ok := r.open[slug]; ok {
		if err := sp.db.Close(); err != nil {
			return fmt.Errorf("fechar espaço %q: %w", slug, err)
		}
		delete(r.open, slug)
	}
	if err := r.moveToTrash(slug); err != nil {
		return err
	}
	i := c.find(slug)
	c.Spaces = append(c.Spaces[:i], c.Spaces[i+1:]...)
	return writeCatalog(r.catalogPath(), c)
}

func (r *Registry) moveToTrash(slug string) error {
	src := r.spacePath(slug)
	if _, err := os.Stat(src); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	trash := filepath.Join(r.spacesDir(), "trash")
	if err := os.MkdirAll(trash, 0o755); err != nil {
		return fmt.Errorf("criar lixeira de espaços: %w", err)
	}
	dst := filepath.Join(trash, fmt.Sprintf("%s-%s.db", slug, time.Now().UTC().Format("20060102-150405")))
	for n := 2; ; n++ {
		if _, err := os.Stat(dst); errors.Is(err, os.ErrNotExist) {
			break
		}
		dst = filepath.Join(trash, fmt.Sprintf("%s-%s-%d.db", slug, time.Now().UTC().Format("20060102-150405"), n))
	}
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("mover espaço %q para a lixeira: %w", slug, err)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(src + suffix); err == nil {
			_ = os.Rename(src+suffix, dst+suffix)
		}
	}
	return nil
}

// removeDBFiles apaga um banco que o próprio registro acabou de criar (falha ao gravar o catálogo).
func removeDBFiles(path string) {
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		_ = os.Remove(p)
	}
}

// Overview é a visão unificada: os totais de cada espaço e a soma de todos.
func (r *Registry) Overview(ctx context.Context) (domain.SpacesOverview, error) {
	list, err := r.List()
	if err != nil {
		return domain.SpacesOverview{}, err
	}
	out := domain.SpacesOverview{Spaces: make([]domain.SpaceOverviewItem, 0, len(list))}
	for _, sp := range list {
		svcs, err := r.Services(ctx, sp.Slug)
		if err != nil {
			return domain.SpacesOverview{}, err
		}
		totals, err := svcs.SpaceTotals(ctx)
		if err != nil {
			return domain.SpacesOverview{}, err
		}
		out.Spaces = append(out.Spaces, domain.SpaceOverviewItem{Slug: sp.Slug, Name: sp.Name, Kind: sp.Kind, Color: sp.Color, SpaceTotals: totals})
		out.Totals.Add(totals)
	}
	return out, nil
}

// Close fecha os bancos dos espaços abertos (o pessoal é de quem criou o registro).
func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	var errs []error
	for slug, sp := range r.open {
		if err := sp.db.Close(); err != nil {
			errs = append(errs, fmt.Errorf("fechar espaço %q: %w", slug, err))
		}
		delete(r.open, slug)
	}
	return errors.Join(errs...)
}
