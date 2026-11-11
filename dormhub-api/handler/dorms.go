package handler

import (
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"dormhub/config"
	"dormhub/store"
)

// ---------- Guest: ใครก็ดูได้ ไม่ต้อง login ----------

// ListZones GET /api/zones
func (h *Handler) ListZones(w http.ResponseWriter, r *http.Request) {
	items, err := h.Store.ListZones(r.Context())
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// ListDorms GET /api/dorms?q=&zone=&gender=&priceMin=&priceMax=&available=1&sort=&limit=
func (h *Handler) ListDorms(w http.ResponseWriter, r *http.Request) {
	h.listDorms(w, r, false)
}

// GetDorm GET /api/dorms/{id}
func (h *Handler) GetDorm(w http.ResponseWriter, r *http.Request) {
	h.getDorm(w, r, false)
}

// parseFilter อ่านตัวกรองจาก query string ค่าที่อ่านไม่ออกจะถือว่าไม่ได้กรอง
func parseFilter(q url.Values) (store.DormFilter, string) {
	f := store.DormFilter{
		Q:             strings.TrimSpace(q.Get("q")),
		Zone:          q.Get("zone"),
		Gender:        q.Get("gender"),
		AvailableOnly: q.Get("available") == "1" || q.Get("available") == "true",
		Sort:          q.Get("sort"),
		Limit:         config.DefaultPageSize,
	}
	if f.Gender != "" && !config.Genders[f.Gender] {
		return f, "ประเภทหอต้องเป็น mixed, female หรือ male"
	}
	if f.Sort != "" && !store.ValidSort(f.Sort) {
		return f, "รูปแบบการเรียงไม่ถูกต้อง"
	}
	var err error
	if v := q.Get("priceMin"); v != "" {
		if f.PriceMin, err = strconv.Atoi(v); err != nil || f.PriceMin < 0 {
			return f, "ราคาต่ำสุดต้องเป็นตัวเลข"
		}
	}
	if v := q.Get("priceMax"); v != "" {
		if f.PriceMax, err = strconv.Atoi(v); err != nil || f.PriceMax < 0 {
			return f, "ราคาสูงสุดต้องเป็นตัวเลข"
		}
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return f, "limit ต้องเป็นตัวเลขตั้งแต่ 1"
		}
		f.Limit = min(n, config.MaxPageSize)
	}
	return f, ""
}

func (h *Handler) listDorms(w http.ResponseWriter, r *http.Request, admin bool) {
	f, msg := parseFilter(r.URL.Query())
	if msg != "" {
		writeError(w, http.StatusBadRequest, "BAD_FILTER", msg)
		return
	}
	f.IncludeHidden = admin
	if admin {
		f.Limit = config.MaxPageSize * 10 // หน้า Admin ต้องเห็นครบทุกหอ
		if f.Sort == "" {
			f.Sort = "newest"
		}
	}
	items, err := h.Store.ListDorms(r.Context(), f)
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) getDorm(w http.ResponseWriter, r *http.Request, admin bool) {
	id, ok := pathID(r, "id")
	if !ok {
		badID(w)
		return
	}
	d, err := h.Store.GetDorm(r.Context(), id, admin)
	if isNotFound(err) {
		notFound(w, "หอพักนี้")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// ---------- Admin: จัดการหอพัก ----------

func (h *Handler) AdminListDorms(w http.ResponseWriter, r *http.Request) { h.listDorms(w, r, true) }
func (h *Handler) AdminGetDorm(w http.ResponseWriter, r *http.Request)   { h.getDorm(w, r, true) }

// AdminCreateDorm POST /api/admin/dorms
func (h *Handler) AdminCreateDorm(w http.ResponseWriter, r *http.Request) {
	var in store.DormInput
	if !readJSON(w, r, &in) {
		return
	}
	if msg := cleanDorm(&in); msg != "" {
		writeError(w, http.StatusBadRequest, "VALIDATION", msg)
		return
	}
	id, err := h.Store.CreateDorm(r.Context(), in)
	if errors.Is(err, store.ErrBadReference) {
		writeError(w, http.StatusBadRequest, "BAD_ZONE", "ไม่พบย่านที่เลือก")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	d, err := h.Store.GetDorm(r.Context(), id, true)
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, d)
}

// AdminUpdateDorm PUT /api/admin/dorms/{id}
func (h *Handler) AdminUpdateDorm(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		badID(w)
		return
	}
	var in store.DormInput
	if !readJSON(w, r, &in) {
		return
	}
	if msg := cleanDorm(&in); msg != "" {
		writeError(w, http.StatusBadRequest, "VALIDATION", msg)
		return
	}
	err := h.Store.UpdateDorm(r.Context(), id, in)
	switch {
	case isNotFound(err):
		notFound(w, "หอพักนี้")
		return
	case errors.Is(err, store.ErrBadReference):
		writeError(w, http.StatusBadRequest, "BAD_REFERENCE", "ย่านหรือประเภทห้องที่ส่งมาไม่ถูกต้อง")
		return
	case err != nil:
		internal(w, err)
		return
	}
	d, err := h.Store.GetDorm(r.Context(), id, true)
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// AdminDeleteDorm DELETE /api/admin/dorms/{id}  (รีวิว รายการโปรด คำขอของหอนี้ถูกลบตามด้วย CASCADE)
func (h *Handler) AdminDeleteDorm(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		badID(w)
		return
	}
	err := h.Store.DeleteDorm(r.Context(), id)
	if isNotFound(err) {
		notFound(w, "หอพักนี้")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// cleanDorm ตัดช่องว่างและตรวจข้อมูลหอ คืน "" ถ้าผ่าน
func cleanDorm(in *store.DormInput) string {
	for _, p := range []*string{&in.Name, &in.Gender, &in.Address, &in.Nearby, &in.Description,
		&in.Phone, &in.LineID, &in.WaterRate, &in.ElectricRate, &in.Deposit} {
		*p = strings.TrimSpace(*p)
	}
	switch {
	case runeLen(in.Name) < 2 || runeLen(in.Name) > 150:
		return "ชื่อหอต้องยาว 2-150 ตัวอักษร"
	case in.ZoneID <= 0:
		return "กรุณาเลือกย่าน"
	case !config.Genders[in.Gender]:
		return "ประเภทหอต้องเป็น รวม / หญิง / ชาย"
	case in.Address == "" || runeLen(in.Address) > 300:
		return "กรุณากรอกที่อยู่ (ไม่เกิน 300 ตัวอักษร)"
	case runeLen(in.Nearby) > 200:
		return "สถานที่ใกล้เคียงยาวเกิน 200 ตัวอักษร"
	case runeLen(in.LineID) > 50 || runeLen(in.WaterRate) > 60 || runeLen(in.ElectricRate) > 60 || runeLen(in.Deposit) > 100:
		return "ข้อมูลค่าใช้จ่ายหรือ LINE ยาวเกินไป"
	}
	if msg := checkPhone(in.Phone, false); msg != "" {
		return msg
	}
	// สิ่งอำนวยความสะดวก: ตัดช่องว่าง ตัดตัวซ้ำ ตัดค่าว่าง และต้องไม่เป็น nil (คอลัมน์เป็น NOT NULL)
	seen := map[string]bool{}
	facilities := []string{}
	for _, f := range in.Facilities {
		f = strings.TrimSpace(f)
		if f == "" || seen[f] {
			continue
		}
		if runeLen(f) > 40 {
			return "ชื่อสิ่งอำนวยความสะดวกยาวเกิน 40 ตัวอักษร"
		}
		seen[f] = true
		facilities = append(facilities, f)
	}
	in.Facilities = facilities
	if msg := cleanImages(in); msg != "" {
		return msg
	}
	if len(in.Rooms) > 20 {
		return "ใส่ประเภทห้องได้ไม่เกิน 20 แบบ"
	}
	for i := range in.Rooms {
		room := &in.Rooms[i]
		room.Name = strings.TrimSpace(room.Name)
		switch {
		case room.Name == "" || runeLen(room.Name) > 100:
			return "ประเภทห้องที่ " + strconv.Itoa(i+1) + ": กรุณากรอกชื่อ"
		case room.PriceMonth <= 0 || room.PriceMonth > 1_000_000:
			return "ประเภทห้องที่ " + strconv.Itoa(i+1) + ": ราคาต้องมากกว่า 0"
		case room.Available < 0 || room.Available > 10_000:
			return "ประเภทห้องที่ " + strconv.Itoa(i+1) + ": จำนวนห้องว่างไม่ถูกต้อง"
		case room.SizeSqm != nil && (*room.SizeSqm <= 0 || *room.SizeSqm > 9999):
			return "ประเภทห้องที่ " + strconv.Itoa(i+1) + ": ขนาดห้องไม่ถูกต้อง"
		}
	}
	return ""
}

// localPhoto ไฟล์รูปที่วางไว้ใน public/photos ของหน้าเว็บ เช่น /photos/d01-front.jpg
// ยอมเฉพาะตัวอักษรอังกฤษ ตัวเลข . _ - และนามสกุลรูป (กันชื่อแปลก ๆ อย่าง ../ )
var localPhoto = regexp.MustCompile(`(?i)^/photos/[a-z0-9][a-z0-9_-]*(\.[a-z0-9_-]+)*\.(jpe?g|png|webp)$`)

// cleanImages ตัดช่องว่าง ข้ามแถวที่ไม่ได้ใส่ลิงก์ ตัดลิงก์ซ้ำ และตรวจว่าเป็น https
// (บังคับ https หรือไฟล์ใน /photos/ เพราะหน้าเว็บบน Vercel เป็น https รูป http จะถูกเบราว์เซอร์บล็อก
// และกันลิงก์แปลก ๆ อย่าง javascript: ที่อาจถูกใช้โจมตี)
func cleanImages(in *store.DormInput) string {
	seen := map[string]bool{}
	images := []store.ImageInput{}
	for _, im := range in.Images {
		im.URL = strings.TrimSpace(im.URL)
		im.Caption = strings.TrimSpace(im.Caption)
		if im.URL == "" || seen[im.URL] {
			continue
		}
		n := strconv.Itoa(len(images) + 1)
		switch {
		case !strings.HasPrefix(im.URL, "https://") && !localPhoto.MatchString(im.URL):
			return "รูปที่ " + n + ": ลิงก์ต้องขึ้นต้นด้วย https:// หรือเป็นไฟล์ /photos/ชื่อไฟล์.jpg"
		case len(im.URL) > 500:
			return "รูปที่ " + n + ": ลิงก์ยาวเกิน 500 ตัวอักษร"
		case runeLen(im.Caption) > 100:
			return "รูปที่ " + n + ": คำอธิบายยาวเกิน 100 ตัวอักษร"
		}
		seen[im.URL] = true
		images = append(images, im)
	}
	if len(images) > config.MaxImages {
		return "ใส่รูปได้ไม่เกิน " + strconv.Itoa(config.MaxImages) + " รูปต่อหอ"
	}
	in.Images = images
	return ""
}
