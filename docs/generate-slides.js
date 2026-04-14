const pptxgen = require("pptxgenjs");
const fs = require("fs");

const pres = new pptxgen();
pres.layout = "LAYOUT_16x9";
pres.author = "Log Analytics Team";
pres.title = "He thong Phan tich Log va Phat hien Bat thuong Thoi gian thuc";

// ── Design Tokens ──────────────────────────────────────────────────────────
const C = {
  dark:     "0B1929",
  darkBlue: "122B45",
  midBlue:  "1A3F6F",
  blue:     "2B7DE9",
  accent:   "FF6B35",
  teal:     "00BFA5",
  green:    "4CAF50",
  red:      "EF5350",
  yellow:   "FFC107",
  white:    "FFFFFF",
  light:    "F0F4FA",
  gray:     "94A3B8",
  grayDark: "64748B",
  textDim:  "CBD5E1",
  card:     "162A46",
  cardBorder: "1E3A5F",
};

const FONT_H = "Calibri";
const FONT_B = "Calibri";

// ── Helpers ─────────────────────────────────────────────────────────────────

function shadow() {
  return { type: "outer", blur: 8, offset: 2, angle: 135, color: "000000", opacity: 0.25 };
}

function darkSlide(slide) {
  slide.background = { color: C.dark };
}

function lightSlide(slide) {
  slide.background = { color: C.light };
}

function addSlideNumber(slide, num, total) {
  slide.addText(`${num} / ${total}`, {
    x: 8.8, y: 5.2, w: 1, h: 0.3,
    fontSize: 9, fontFace: FONT_B, color: C.gray, align: "right",
  });
}

function addTopBar(slide) {
  slide.addShape(pres.shapes.RECTANGLE, {
    x: 0, y: 0, w: 10, h: 0.06, fill: { color: C.blue },
  });
}

function addBottomAccent(slide) {
  slide.addShape(pres.shapes.RECTANGLE, {
    x: 0, y: 5.525, w: 10, h: 0.1, fill: { color: C.blue },
  });
}

function addSectionTitle(slide, section) {
  slide.addText(section, {
    x: 0.6, y: 0.2, w: 5, h: 0.3,
    fontSize: 9, fontFace: FONT_B, color: C.gray, italic: true,
  });
}

function card(slide, x, y, w, h, bgColor) {
  slide.addShape(pres.shapes.RECTANGLE, {
    x, y, w, h, fill: { color: bgColor || C.card },
    shadow: shadow(),
    line: { color: C.cardBorder, width: 0.5 },
  });
}

function iconCircle(slide, x, y, size, color, label) {
  slide.addShape(pres.shapes.OVAL, {
    x, y, w: size, h: size, fill: { color },
  });
  if (label) {
    slide.addText(label, {
      x, y, w: size, h: size,
      fontSize: Math.round(size * 14), fontFace: FONT_H, color: C.white,
      align: "center", valign: "middle", bold: true,
    });
  }
}

const TOTAL = 25;

// ============================================================================
// SLIDE 1: Title
// ============================================================================
{
  const s = pres.addSlide();
  s.background = { color: C.dark };
  s.addShape(pres.shapes.RECTANGLE, {
    x: 0, y: 0, w: 10, h: 5.625,
    fill: { color: C.darkBlue, transparency: 30 },
  });
  s.addShape(pres.shapes.RECTANGLE, {
    x: 0, y: 4.8, w: 10, h: 0.825, fill: { color: C.blue, transparency: 60 },
  });
  s.addShape(pres.shapes.RECTANGLE, {
    x: 0.6, y: 1.4, w: 0.08, h: 2.2, fill: { color: C.accent },
  });

  s.addText("HE THONG PHAN TICH LOG\nVA PHAT HIEN BAT THUONG\nTHOI GIAN THUC", {
    x: 1.0, y: 1.3, w: 8, h: 2.4,
    fontSize: 36, fontFace: FONT_H, color: C.white, bold: true,
    lineSpacingMultiple: 1.1,
  });
  s.addText("Real-time Log Analytics & Anomaly Detection System", {
    x: 1.0, y: 3.6, w: 8, h: 0.5,
    fontSize: 16, fontFace: FONT_B, color: C.textDim, italic: true,
  });

  s.addText([
    { text: "Go  |  Apache Kafka  |  Elasticsearch  |  Kibana  |  Grafana  |  Prometheus", options: { breakLine: true } },
    { text: "Event-Driven Pipeline  |  Sliding Window Detection", options: {} },
  ], {
    x: 0.6, y: 4.9, w: 7, h: 0.6,
    fontSize: 11, fontFace: FONT_B, color: C.white,
  });

  s.addText("Thang 4/2026", {
    x: 7.5, y: 5.0, w: 2, h: 0.4,
    fontSize: 12, fontFace: FONT_B, color: C.white, align: "right",
  });
}

// ============================================================================
// SLIDE 2: Muc luc
// ============================================================================
{
  const s = pres.addSlide();
  darkSlide(s); addTopBar(s); addSlideNumber(s, 2, TOTAL);

  s.addText("MUC LUC", {
    x: 0.6, y: 0.3, w: 4, h: 0.7,
    fontSize: 32, fontFace: FONT_H, color: C.white, bold: true, margin: 0,
  });

  const chapters = [
    ["01", "Gioi thieu", C.blue],
    ["02", "Kien truc tong the he thong", C.blue],
    ["03", "Cong nghe su dung", C.teal],
    ["04", "Pipeline xu ly du lieu", C.teal],
    ["05", "Cac thuat toan phat hien bat thuong", C.accent],
    ["06", "Toi uu hieu nang va kha nang chiu tai", C.accent],
    ["07", "He thong giam sat va quan sat", C.green],
    ["08", "Kiem thu va dam bao chat luong", C.green],
    ["09", "Ket luan va huong phat trien", C.gray],
  ];

  chapters.forEach(([num, title, color], i) => {
    const yy = 1.3 + i * 0.45;
    s.addShape(pres.shapes.RECTANGLE, {
      x: 0.6, y: yy, w: 0.55, h: 0.35, fill: { color },
    });
    s.addText(num, {
      x: 0.6, y: yy, w: 0.55, h: 0.35,
      fontSize: 12, fontFace: FONT_H, color: C.white, align: "center", valign: "middle", bold: true,
    });
    s.addText(title, {
      x: 1.35, y: yy, w: 7, h: 0.35,
      fontSize: 15, fontFace: FONT_B, color: C.white, valign: "middle",
    });
  });
}

// ============================================================================
// SLIDE 3: Boi canh va dong luc
// ============================================================================
{
  const s = pres.addSlide();
  darkSlide(s); addTopBar(s); addSlideNumber(s, 3, TOTAL);
  addSectionTitle(s, "01 — Gioi thieu");

  s.addText("Boi canh va dong luc", {
    x: 0.6, y: 0.5, w: 8, h: 0.6,
    fontSize: 28, fontFace: FONT_H, color: C.white, bold: true, margin: 0,
  });

  // Left column - pain points
  card(s, 0.6, 1.4, 4.2, 3.6, C.card);
  s.addShape(pres.shapes.RECTANGLE, {
    x: 0.6, y: 1.4, w: 0.06, h: 3.6, fill: { color: C.red },
  });
  s.addText("Thach thuc", {
    x: 0.9, y: 1.5, w: 3.5, h: 0.4,
    fontSize: 16, fontFace: FONT_H, color: C.red, bold: true,
  });
  s.addText([
    { text: "Kien truc microservice sinh ra hang trieu ban ghi log moi phut", options: { bullet: true, breakLine: true, color: C.white } },
    { text: "Phan tich thu cong bat kha thi ve thoi gian va nhan luc", options: { bullet: true, breakLine: true, color: C.white } },
    { text: "Tan cong bao mat, suy giam hieu nang can phat hien ngay lap tuc", options: { bullet: true, breakLine: true, color: C.white } },
    { text: "Cac he thong phan tan co hang chuc dich vu chay dong thoi", options: { bullet: true, color: C.white } },
  ], {
    x: 0.9, y: 2.0, w: 3.7, h: 2.8,
    fontSize: 12, fontFace: FONT_B, color: C.white, paraSpaceAfter: 8,
  });

  // Right column - solution
  card(s, 5.2, 1.4, 4.2, 3.6, C.card);
  s.addShape(pres.shapes.RECTANGLE, {
    x: 5.2, y: 1.4, w: 0.06, h: 3.6, fill: { color: C.teal },
  });
  s.addText("Giai phap", {
    x: 5.5, y: 1.5, w: 3.5, h: 0.4,
    fontSize: 16, fontFace: FONT_H, color: C.teal, bold: true,
  });
  s.addText([
    { text: "Tiep nhan log qua Apache Kafka (hang doi phan tan)", options: { bullet: true, breakLine: true, color: C.white } },
    { text: "Pipeline da luong song song xu ly thoi gian thuc", options: { bullet: true, breakLine: true, color: C.white } },
    { text: "6 quy tac phat hien bat thuong dua tren cua so truot", options: { bullet: true, breakLine: true, color: C.white } },
    { text: "Luu tru Elasticsearch, truc quan hoa Kibana + Grafana", options: { bullet: true, color: C.white } },
  ], {
    x: 5.5, y: 2.0, w: 3.7, h: 2.8,
    fontSize: 12, fontFace: FONT_B, color: C.white, paraSpaceAfter: 8,
  });
}

// ============================================================================
// SLIDE 4: Muc tieu he thong
// ============================================================================
{
  const s = pres.addSlide();
  darkSlide(s); addTopBar(s); addSlideNumber(s, 4, TOTAL);
  addSectionTitle(s, "01 — Gioi thieu");

  s.addText("Muc tieu he thong", {
    x: 0.6, y: 0.5, w: 8, h: 0.6,
    fontSize: 28, fontFace: FONT_H, color: C.white, bold: true, margin: 0,
  });

  const goals = [
    [C.blue, "Thong luong cao", "Xu ly hang tram nghin ban ghi log/giay.\nAt-least-once delivery, idempotent processing.\nKhong mat du lieu."],
    [C.accent, "Phat hien tu dong", "6 loai bat thuong: error spike, latency breach,\nrepeated failure, auth burst, off-hours access,\nservice silence."],
    [C.teal, "Quan sat toan dien", "Kibana duyet log, Grafana dashboard metric,\nPrometheus thu thap chi so hieu nang.\nTheo doi thoi gian thuc."],
  ];

  goals.forEach(([color, title, desc], i) => {
    const yy = 1.4 + i * 1.3;
    card(s, 0.6, yy, 8.8, 1.1, C.card);
    s.addShape(pres.shapes.RECTANGLE, {
      x: 0.6, y: yy, w: 0.06, h: 1.1, fill: { color },
    });
    iconCircle(s, 0.9, yy + 0.2, 0.65, color, `0${i + 1}`);
    s.addText(title, {
      x: 1.8, y: yy + 0.1, w: 7, h: 0.35,
      fontSize: 16, fontFace: FONT_H, color, bold: true,
    });
    s.addText(desc, {
      x: 1.8, y: yy + 0.45, w: 7.3, h: 0.6,
      fontSize: 11, fontFace: FONT_B, color: C.textDim,
    });
  });
}

// ============================================================================
// SLIDE 5: Kien truc tong the
// ============================================================================
{
  const s = pres.addSlide();
  darkSlide(s); addTopBar(s); addSlideNumber(s, 5, TOTAL);
  addSectionTitle(s, "02 — Kien truc tong the");

  s.addText("Kien truc huong su kien (Event-Driven)", {
    x: 0.6, y: 0.5, w: 9, h: 0.6,
    fontSize: 28, fontFace: FONT_H, color: C.white, bold: true, margin: 0,
  });

  // Four layers - horizontal flow
  const layers = [
    [C.accent, "Tiep nhan\n(Ingestion)", "Apache Kafka\n3 partitions\nBuffer phan tan"],
    [C.blue, "Xu ly\n(Processing)", "Go pipeline\n4 worker goroutines\nDetection Engine"],
    [C.teal, "Luu tru\n(Storage)", "Elasticsearch\nlogs-YYYY.MM.DD\nanomalies index"],
    [C.green, "Truc quan hoa\n(Visualization)", "Kibana + Grafana\nPrometheus\nDashboard"],
  ];

  layers.forEach(([color, title, desc], i) => {
    const xx = 0.4 + i * 2.4;
    card(s, xx, 1.5, 2.1, 3.2, C.card);
    s.addShape(pres.shapes.RECTANGLE, {
      x: xx, y: 1.5, w: 2.1, h: 0.06, fill: { color },
    });
    s.addText(title, {
      x: xx + 0.15, y: 1.7, w: 1.8, h: 0.8,
      fontSize: 13, fontFace: FONT_H, color, bold: true, align: "center",
    });
    s.addText(desc, {
      x: xx + 0.15, y: 2.6, w: 1.8, h: 1.8,
      fontSize: 11, fontFace: FONT_B, color: C.textDim, align: "center",
    });

    // Arrow between layers
    if (i < 3) {
      s.addShape(pres.shapes.LINE, {
        x: xx + 2.15, y: 3.1, w: 0.2, h: 0,
        line: { color: C.gray, width: 2 },
      });
    }
  });

  s.addText("Du lieu di chuyen mot chieu tu trai sang phai. Producer va consumer tach roi hoan toan qua Kafka.", {
    x: 0.6, y: 4.9, w: 8, h: 0.4,
    fontSize: 10, fontFace: FONT_B, color: C.gray, italic: true,
  });
}

// ============================================================================
// SLIDE 6: Cac thanh phan chinh
// ============================================================================
{
  const s = pres.addSlide();
  darkSlide(s); addTopBar(s); addSlideNumber(s, 6, TOTAL);
  addSectionTitle(s, "02 — Kien truc tong the");

  s.addText("Cac thanh phan chinh", {
    x: 0.6, y: 0.5, w: 8, h: 0.6,
    fontSize: 28, fontFace: FONT_H, color: C.white, bold: true, margin: 0,
  });

  const rows = [
    [{ text: "Thanh phan", options: { bold: true, color: C.white, fill: { color: C.midBlue } } },
     { text: "Cong nghe", options: { bold: true, color: C.white, fill: { color: C.midBlue } } },
     { text: "Port", options: { bold: true, color: C.white, fill: { color: C.midBlue } } },
     { text: "Vai tro", options: { bold: true, color: C.white, fill: { color: C.midBlue } } }],
    ["Message Queue", "Kafka 7.5 (KRaft)", "9092", "Hang doi log, 3 partitions"],
    ["Storage", "Elasticsearch 9.0", "9200", "Luu tru & truy van log + anomaly"],
    ["Core App", "Go 1.25", "8080, 2112", "Pipeline xu ly & phat hien"],
    ["Log Viewer", "Kibana 9.0", "5601", "Giao dien duyet log (ELK)"],
    ["Dashboard", "Grafana 12.0", "3000", "Dashboard metric"],
    ["Metrics", "Prometheus 3.4", "9090", "Thu thap metric"],
    ["Init Containers", "confluent + curl", "-", "Tao topic & data view tu dong"],
  ];

  s.addTable(rows, {
    x: 0.6, y: 1.3, w: 8.8, colW: [1.8, 2.0, 1.2, 3.8],
    fontSize: 11, fontFace: FONT_B, color: C.white,
    border: { pt: 0.5, color: C.cardBorder },
    rowH: [0.4, 0.4, 0.4, 0.4, 0.4, 0.4, 0.4, 0.4],
    autoPage: false,
    fill: { color: C.card },
    altColor: C.darkBlue,
  });

  s.addText("Toan bo 8 container duoc dieu phoi boi Docker Compose. Health check dam bao thu tu khoi dong dung.", {
    x: 0.6, y: 4.9, w: 8.8, h: 0.4,
    fontSize: 10, fontFace: FONT_B, color: C.gray, italic: true,
  });
}

// ============================================================================
// SLIDE 7: Apache Kafka & KRaft
// ============================================================================
{
  const s = pres.addSlide();
  darkSlide(s); addTopBar(s); addSlideNumber(s, 7, TOTAL);
  addSectionTitle(s, "03 — Cong nghe su dung");

  s.addText("Apache Kafka & mo hinh KRaft", {
    x: 0.6, y: 0.5, w: 9, h: 0.6,
    fontSize: 28, fontFace: FONT_H, color: C.white, bold: true, margin: 0,
  });

  // Left: KRaft explanation
  card(s, 0.6, 1.3, 4.3, 2.0, C.card);
  s.addText("KRaft — Loai bo ZooKeeper", {
    x: 0.9, y: 1.4, w: 3.8, h: 0.35,
    fontSize: 14, fontFace: FONT_H, color: C.accent, bold: true,
  });
  s.addText([
    { text: "Giao thuc dong thuan Raft tich hop trong broker", options: { bullet: true, breakLine: true } },
    { text: "Don gian hoa trieu khai va van hanh", options: { bullet: true, breakLine: true } },
    { text: "Giam so luong thanh phan can quan ly", options: { bullet: true } },
  ], {
    x: 0.9, y: 1.85, w: 3.8, h: 1.3,
    fontSize: 11, fontFace: FONT_B, color: C.textDim, paraSpaceAfter: 6,
  });

  // Right: Config
  card(s, 5.2, 1.3, 4.2, 2.0, C.card);
  s.addText("Cau hinh toi uu", {
    x: 5.5, y: 1.4, w: 3.8, h: 0.35,
    fontSize: 14, fontFace: FONT_H, color: C.blue, bold: true,
  });
  s.addText([
    { text: "3 partitions — xu ly song song x3", options: { bullet: true, breakLine: true } },
    { text: "4 luong I/O, 3 luong mang", options: { bullet: true, breakLine: true } },
    { text: "Socket buffer 1MB (gui & nhan)", options: { bullet: true } },
  ], {
    x: 5.5, y: 1.85, w: 3.7, h: 1.3,
    fontSize: 11, fontFace: FONT_B, color: C.textDim, paraSpaceAfter: 6,
  });

  // Bottom: At-least-once
  card(s, 0.6, 3.6, 8.8, 1.5, C.card);
  s.addShape(pres.shapes.RECTANGLE, {
    x: 0.6, y: 3.6, w: 8.8, h: 0.06, fill: { color: C.teal },
  });
  s.addText("At-Least-Once Delivery", {
    x: 0.9, y: 3.7, w: 4, h: 0.35,
    fontSize: 14, fontFace: FONT_H, color: C.teal, bold: true,
  });
  s.addText("Offset chi duoc commit sau khi message vao Go channel thanh cong. Neu consumer gap su co, message duoc xu ly lai. SHA-256 document ID trong Elasticsearch dam bao idempotent — xu ly lai khong tao ban ghi trung lap.", {
    x: 0.9, y: 4.1, w: 8.3, h: 0.8,
    fontSize: 11, fontFace: FONT_B, color: C.textDim,
  });
}

// ============================================================================
// SLIDE 8: Elasticsearch
// ============================================================================
{
  const s = pres.addSlide();
  darkSlide(s); addTopBar(s); addSlideNumber(s, 8, TOTAL);
  addSectionTitle(s, "03 — Cong nghe su dung");

  s.addText("Elasticsearch & chien luoc danh muc", {
    x: 0.6, y: 0.5, w: 9, h: 0.6,
    fontSize: 28, fontFace: FONT_H, color: C.white, bold: true, margin: 0,
  });

  // Two index strategies
  const boxes = [
    [C.blue, "logs-YYYY.MM.DD", "Phan theo ngay\nDe dang xoa index cu\nQuan ly vong doi du lieu"],
    [C.accent, "anomalies", "Index co dinh duy nhat\nLuong anomaly thap\nKhong can phan vung"],
  ];

  boxes.forEach(([color, title, desc], i) => {
    const xx = 0.6 + i * 4.6;
    card(s, xx, 1.3, 4.2, 1.5, C.card);
    s.addShape(pres.shapes.RECTANGLE, {
      x: xx, y: 1.3, w: 0.06, h: 1.5, fill: { color },
    });
    s.addText(title, {
      x: xx + 0.3, y: 1.4, w: 3.6, h: 0.35,
      fontSize: 14, fontFace: "Consolas", color, bold: true,
    });
    s.addText(desc, {
      x: xx + 0.3, y: 1.8, w: 3.6, h: 0.8,
      fontSize: 11, fontFace: FONT_B, color: C.textDim,
    });
  });

  // Mapping strategy
  card(s, 0.6, 3.1, 8.8, 2.2, C.card);
  s.addText("dynamic: false — Chi danh muc truong duoc dinh nghia truoc", {
    x: 0.9, y: 3.2, w: 8, h: 0.35,
    fontSize: 14, fontFace: FONT_H, color: C.teal, bold: true,
  });

  const fields = [
    ["@timestamp", "date", "Truy van theo khoang thoi gian"],
    ["level, service", "keyword", "Loc chinh xac (term query)"],
    ["message", "text", "Tim kiem toan van (full-text)"],
    ["fields", "flattened", "Truong dong, khong tao mapping rieng"],
  ];

  fields.forEach(([field, type, note], i) => {
    const yy = 3.7 + i * 0.38;
    s.addText(field, {
      x: 0.9, y: yy, w: 2.0, h: 0.33,
      fontSize: 11, fontFace: "Consolas", color: C.yellow, valign: "middle",
    });
    s.addText(type, {
      x: 3.0, y: yy, w: 1.3, h: 0.33,
      fontSize: 11, fontFace: FONT_B, color: C.blue, valign: "middle",
    });
    s.addText(note, {
      x: 4.4, y: yy, w: 4.8, h: 0.33,
      fontSize: 11, fontFace: FONT_B, color: C.textDim, valign: "middle",
    });
  });
}

// ============================================================================
// SLIDE 9: Go & Concurrency
// ============================================================================
{
  const s = pres.addSlide();
  darkSlide(s); addTopBar(s); addSlideNumber(s, 9, TOTAL);
  addSectionTitle(s, "03 — Cong nghe su dung");

  s.addText("Go & Mo hinh dong thoi", {
    x: 0.6, y: 0.5, w: 9, h: 0.6,
    fontSize: 28, fontFace: FONT_H, color: C.white, bold: true, margin: 0,
  });

  // Goroutine vs Thread comparison
  card(s, 0.6, 1.3, 4.2, 1.8, C.card);
  s.addText("Goroutine vs Thread", {
    x: 0.9, y: 1.4, w: 3.5, h: 0.35,
    fontSize: 14, fontFace: FONT_H, color: C.accent, bold: true,
  });
  s.addText([
    { text: "Stack ban dau: vai KB (vs MB cho thread)", options: { bullet: true, breakLine: true } },
    { text: "Chi phi chuyen doi ngu canh cuc thap", options: { bullet: true, breakLine: true } },
    { text: "Hang tram goroutine dong thoi", options: { bullet: true, breakLine: true } },
    { text: "Channel: truyen tin nhan an toan, khong mutex", options: { bullet: true } },
  ], {
    x: 0.9, y: 1.85, w: 3.7, h: 1.2,
    fontSize: 11, fontFace: FONT_B, color: C.textDim, paraSpaceAfter: 4,
  });

  // Libraries
  card(s, 5.2, 1.3, 4.2, 1.8, C.card);
  s.addText("Thu vien chinh", {
    x: 5.5, y: 1.4, w: 3.5, h: 0.35,
    fontSize: 14, fontFace: FONT_H, color: C.blue, bold: true,
  });

  const libs = [
    ["franz-go", "Kafka client thuan Go"],
    ["chi", "HTTP router + middleware"],
    ["go-elasticsearch v9", "ES client + BulkIndexer"],
    ["errgroup", "Dieu phoi goroutine lifecycle"],
  ];
  libs.forEach(([name, desc], i) => {
    const yy = 1.85 + i * 0.28;
    s.addText(name, {
      x: 5.5, y: yy, w: 2.0, h: 0.25,
      fontSize: 11, fontFace: "Consolas", color: C.teal, valign: "middle",
    });
    s.addText(desc, {
      x: 7.5, y: yy, w: 1.8, h: 0.25,
      fontSize: 10, fontFace: FONT_B, color: C.textDim, valign: "middle",
    });
  });

  // errgroup diagram - simplified goroutine flow
  card(s, 0.6, 3.4, 8.8, 1.8, C.card);
  s.addText("errgroup — Dieu phoi 7 goroutine dong thoi", {
    x: 0.9, y: 3.5, w: 8, h: 0.35,
    fontSize: 14, fontFace: FONT_H, color: C.green, bold: true,
  });

  const goroutines = [
    [C.blue, "G1\nConsumer"],
    [C.accent, "G2-G5\n4 Workers"],
    [C.teal, "G6\nAPI Server"],
    [C.green, "G7\nDispatcher"],
  ];
  goroutines.forEach(([color, label], i) => {
    const xx = 1.0 + i * 2.1;
    s.addShape(pres.shapes.RECTANGLE, {
      x: xx, y: 4.05, w: 1.7, h: 0.9, fill: { color }, shadow: shadow(),
    });
    s.addText(label, {
      x: xx, y: 4.05, w: 1.7, h: 0.9,
      fontSize: 11, fontFace: FONT_H, color: C.white, align: "center", valign: "middle", bold: true,
    });
  });
}

// ============================================================================
// SLIDE 10: He sinh thai quan sat
// ============================================================================
{
  const s = pres.addSlide();
  darkSlide(s); addTopBar(s); addSlideNumber(s, 10, TOTAL);
  addSectionTitle(s, "03 — Cong nghe su dung");

  s.addText("He sinh thai quan sat", {
    x: 0.6, y: 0.5, w: 9, h: 0.6,
    fontSize: 28, fontFace: FONT_H, color: C.white, bold: true, margin: 0,
  });

  const tools = [
    [C.accent, "Kibana 9.0", "Port 5601", "Duyet va tim kiem log chi tiet\nKQL query language\n2 data view tu dong: logs-* & anomalies"],
    [C.blue, "Grafana 12.0", "Port 3000", "Dashboard truc quan hoa metric\nDu lieu tu Prometheus + Elasticsearch\n7 panel: throughput, latency, anomaly..."],
    [C.teal, "Prometheus 3.4", "Port 9090", "Thu thap metric moi 15 giay\n6 metric chinh: consumed, latency,\nanomaly, lag, write errors, parse errors"],
  ];

  tools.forEach(([color, name, port, desc], i) => {
    const yy = 1.3 + i * 1.35;
    card(s, 0.6, yy, 8.8, 1.15, C.card);
    iconCircle(s, 0.9, yy + 0.25, 0.65, color, `${i + 1}`);
    s.addText(name, {
      x: 1.8, y: yy + 0.1, w: 2.5, h: 0.35,
      fontSize: 16, fontFace: FONT_H, color: C.white, bold: true,
    });
    s.addText(port, {
      x: 1.8, y: yy + 0.45, w: 2.5, h: 0.25,
      fontSize: 10, fontFace: "Consolas", color,
    });
    s.addText(desc, {
      x: 4.5, y: yy + 0.1, w: 4.7, h: 0.95,
      fontSize: 11, fontFace: FONT_B, color: C.textDim,
    });
  });
}

// ============================================================================
// SLIDE 11: Pipeline tong quan
// ============================================================================
{
  const s = pres.addSlide();
  darkSlide(s); addTopBar(s); addSlideNumber(s, 11, TOTAL);
  addSectionTitle(s, "04 — Pipeline xu ly du lieu");

  s.addText("Pipeline xu ly du lieu — Tong quan", {
    x: 0.6, y: 0.5, w: 9, h: 0.6,
    fontSize: 28, fontFace: FONT_H, color: C.white, bold: true, margin: 0,
  });

  // Pipeline flow - horizontal
  const steps = [
    [C.accent, "Kafka\nConsumer", "PollFetches()\nfrom 3 partitions"],
    [C.blue, "Channel\n10,000 buffer", "Backpressure\ntự nhiên"],
    [C.teal, "4 Workers\nsong song", "Parse → Index\n→ Detect"],
    [C.green, "BulkIndexer\n10MB / 5s", "Ghi ES\ntheo lô"],
  ];

  steps.forEach(([color, title, desc], i) => {
    const xx = 0.3 + i * 2.5;
    s.addShape(pres.shapes.RECTANGLE, {
      x: xx, y: 1.5, w: 2.1, h: 1.6, fill: { color: C.card },
      shadow: shadow(), line: { color, width: 1.5 },
    });
    s.addText(title, {
      x: xx + 0.1, y: 1.6, w: 1.9, h: 0.7,
      fontSize: 13, fontFace: FONT_H, color, bold: true, align: "center",
    });
    s.addText(desc, {
      x: xx + 0.1, y: 2.3, w: 1.9, h: 0.6,
      fontSize: 10, fontFace: FONT_B, color: C.textDim, align: "center",
    });
    if (i < 3) {
      s.addText("\u2192", {
        x: xx + 2.1, y: 2.0, w: 0.4, h: 0.5,
        fontSize: 20, color: C.gray, align: "center", valign: "middle",
      });
    }
  });

  // Anomaly path
  s.addText("Anomaly Path", {
    x: 0.6, y: 3.5, w: 3, h: 0.35,
    fontSize: 14, fontFace: FONT_H, color: C.red, bold: true,
  });

  const anomSteps = [
    [C.red, "Detection\nEngine", "6 rules evaluate"],
    [C.yellow, "Anomaly Ch\n1,024 buffer", "Cold path"],
    [C.accent, "Dispatcher", "Index + Alert"],
  ];

  anomSteps.forEach(([color, title, desc], i) => {
    const xx = 0.6 + i * 3.1;
    s.addShape(pres.shapes.RECTANGLE, {
      x: xx, y: 3.9, w: 2.7, h: 1.1, fill: { color: C.card },
      line: { color, width: 1 },
    });
    s.addText(title, {
      x: xx + 0.1, y: 3.95, w: 2.5, h: 0.55,
      fontSize: 12, fontFace: FONT_H, color, bold: true, align: "center",
    });
    s.addText(desc, {
      x: xx + 0.1, y: 4.5, w: 2.5, h: 0.35,
      fontSize: 10, fontFace: FONT_B, color: C.textDim, align: "center",
    });
    if (i < 2) {
      s.addText("\u2192", {
        x: xx + 2.7, y: 4.15, w: 0.4, h: 0.5,
        fontSize: 18, color: C.gray, align: "center", valign: "middle",
      });
    }
  });
}

// ============================================================================
// SLIDE 12: Kafka Consumer
// ============================================================================
{
  const s = pres.addSlide();
  darkSlide(s); addTopBar(s); addSlideNumber(s, 12, TOTAL);
  addSectionTitle(s, "04 — Pipeline xu ly du lieu");

  s.addText("Kafka Consumer — Co che doc tin nhan", {
    x: 0.6, y: 0.5, w: 9, h: 0.6,
    fontSize: 28, fontFace: FONT_H, color: C.white, bold: true, margin: 0,
  });

  // Fetch config
  card(s, 0.6, 1.3, 4.2, 2.5, C.card);
  s.addText("Cau hinh Fetch", {
    x: 0.9, y: 1.4, w: 3.5, h: 0.35,
    fontSize: 14, fontFace: FONT_H, color: C.blue, bold: true,
  });

  const fetchConf = [
    ["FetchMaxBytes", "10 MB"],
    ["FetchMaxPartitionBytes", "1 MB"],
    ["MaxConcurrentFetches", "3"],
    ["Channel buffer", "10,000 msgs"],
  ];
  fetchConf.forEach(([k, v], i) => {
    const yy = 1.9 + i * 0.4;
    s.addText(k, {
      x: 0.9, y: yy, w: 2.5, h: 0.33,
      fontSize: 11, fontFace: "Consolas", color: C.textDim, valign: "middle",
    });
    s.addText(v, {
      x: 3.4, y: yy, w: 1.3, h: 0.33,
      fontSize: 12, fontFace: FONT_H, color: C.accent, bold: true, valign: "middle",
    });
  });

  // Safety mechanisms
  card(s, 5.2, 1.3, 4.2, 2.5, C.card);
  s.addText("Co che an toan", {
    x: 5.5, y: 1.4, w: 3.5, h: 0.35,
    fontSize: 14, fontFace: FONT_H, color: C.teal, bold: true,
  });
  s.addText([
    { text: "Offset commit SAU khi vao channel", options: { bullet: true, breakLine: true } },
    { text: "Channel day → consumer bi block (backpressure)", options: { bullet: true, breakLine: true } },
    { text: "Rebalance → OnPartitionsRevoked commit ngay", options: { bullet: true, breakLine: true } },
    { text: "Khong mat message trong moi truong hop", options: { bullet: true } },
  ], {
    x: 5.5, y: 1.9, w: 3.7, h: 1.6,
    fontSize: 11, fontFace: FONT_B, color: C.textDim, paraSpaceAfter: 8,
  });

  // Bottom note
  card(s, 0.6, 4.1, 8.8, 1.1, C.card);
  s.addText("Backpressure tu nhien", {
    x: 0.9, y: 4.2, w: 3, h: 0.3,
    fontSize: 13, fontFace: FONT_H, color: C.yellow, bold: true,
  });
  s.addText("Khi 4 worker xu ly cham hon toc do Kafka gui, channel 10,000 dan day. Consumer tu dong bi block cho den khi co slot trong. Khong can code them — day la co che backpressure san co cua Go channel.", {
    x: 0.9, y: 4.5, w: 8.3, h: 0.6,
    fontSize: 11, fontFace: FONT_B, color: C.textDim,
  });
}

// ============================================================================
// SLIDE 13: Pipeline Workers
// ============================================================================
{
  const s = pres.addSlide();
  darkSlide(s); addTopBar(s); addSlideNumber(s, 13, TOTAL);
  addSectionTitle(s, "04 — Pipeline xu ly du lieu");

  s.addText("4 Worker Goroutines — Xu ly song song", {
    x: 0.6, y: 0.5, w: 9, h: 0.6,
    fontSize: 28, fontFace: FONT_H, color: C.white, bold: true, margin: 0,
  });

  // Channel fan-out to 4 workers
  s.addShape(pres.shapes.RECTANGLE, {
    x: 0.6, y: 1.5, w: 1.5, h: 3.0, fill: { color: C.blue }, shadow: shadow(),
  });
  s.addText("Channel\n10,000\nbuffer", {
    x: 0.6, y: 1.5, w: 1.5, h: 3.0,
    fontSize: 12, fontFace: FONT_H, color: C.white, align: "center", valign: "middle", bold: true,
  });

  // 4 workers
  for (let i = 0; i < 4; i++) {
    const yy = 1.4 + i * 0.8;
    // Connection line
    s.addShape(pres.shapes.LINE, {
      x: 2.1, y: 2.95, w: 0.5, h: 0,
      line: { color: C.gray, width: 1.5 },
    });
    s.addShape(pres.shapes.RECTANGLE, {
      x: 2.8, y: yy, w: 1.6, h: 0.65, fill: { color: C.teal }, shadow: shadow(),
    });
    s.addText(`Worker ${i + 1}`, {
      x: 2.8, y: yy, w: 1.6, h: 0.65,
      fontSize: 11, fontFace: FONT_H, color: C.white, align: "center", valign: "middle", bold: true,
    });
  }

  // Three steps for each worker
  const workerSteps = [
    [C.yellow, "1. Parse", "JSON -> LogEntry\nPlain-text fallback\nneu JSON loi"],
    [C.blue, "2. Index", "BulkIndexer.Add()\nSHA-256 doc ID\nIdempotent write"],
    [C.red, "3. Detect", "engine.Evaluate()\n6 rules check\nAnomaly -> channel"],
  ];

  workerSteps.forEach(([color, title, desc], i) => {
    const xx = 5.0 + i * 1.8;
    card(s, xx, 1.4, 1.6, 3.2, C.card);
    s.addShape(pres.shapes.RECTANGLE, {
      x: xx, y: 1.4, w: 1.6, h: 0.06, fill: { color },
    });
    s.addText(title, {
      x: xx + 0.1, y: 1.55, w: 1.4, h: 0.35,
      fontSize: 12, fontFace: FONT_H, color, bold: true, align: "center",
    });
    s.addText(desc, {
      x: xx + 0.1, y: 2.0, w: 1.4, h: 2.2,
      fontSize: 10, fontFace: FONT_B, color: C.textDim, align: "center",
    });
  });

  s.addText("Go runtime tu dong phan phoi message den worker nao dang san sang — khong can bo phan phoi tuong minh.", {
    x: 0.6, y: 4.9, w: 8.8, h: 0.4,
    fontSize: 10, fontFace: FONT_B, color: C.gray, italic: true,
  });
}

// ============================================================================
// SLIDE 14: Anomaly Dispatcher
// ============================================================================
{
  const s = pres.addSlide();
  darkSlide(s); addTopBar(s); addSlideNumber(s, 14, TOTAL);
  addSectionTitle(s, "04 — Pipeline xu ly du lieu");

  s.addText("Anomaly Dispatcher", {
    x: 0.6, y: 0.5, w: 9, h: 0.6,
    fontSize: 28, fontFace: FONT_H, color: C.white, bold: true, margin: 0,
  });

  // Flow diagram
  const flow = [
    [C.red, "Detection\nEngine", "4 workers\ngoi Evaluate()"],
    [C.yellow, "Anomaly\nChannel", "Buffer 1,024\nCold path"],
    [C.accent, "Dispatcher\nGoroutine", "Doc tuan tu\ntư channel"],
  ];

  flow.forEach(([color, title, desc], i) => {
    const xx = 0.5 + i * 3.3;
    s.addShape(pres.shapes.RECTANGLE, {
      x: xx, y: 1.5, w: 2.8, h: 1.3, fill: { color: C.card },
      line: { color, width: 1.5 }, shadow: shadow(),
    });
    s.addText(title, {
      x: xx, y: 1.55, w: 2.8, h: 0.65,
      fontSize: 14, fontFace: FONT_H, color, bold: true, align: "center", valign: "middle",
    });
    s.addText(desc, {
      x: xx, y: 2.15, w: 2.8, h: 0.55,
      fontSize: 10, fontFace: FONT_B, color: C.textDim, align: "center",
    });
    if (i < 2) {
      s.addText("\u2192", {
        x: xx + 2.8, y: 1.8, w: 0.5, h: 0.5,
        fontSize: 22, color: C.gray, align: "center", valign: "middle",
      });
    }
  });

  // Two actions
  card(s, 0.6, 3.3, 4.2, 1.8, C.card);
  s.addShape(pres.shapes.RECTANGLE, {
    x: 0.6, y: 3.3, w: 0.06, h: 1.8, fill: { color: C.blue },
  });
  s.addText("Hanh dong 1: Luu tru", {
    x: 0.9, y: 3.4, w: 3.5, h: 0.35,
    fontSize: 13, fontFace: FONT_H, color: C.blue, bold: true,
  });
  s.addText("IndexAnomaly() ghi vao ES index \"anomalies\". BulkIndexer flush moi 3 giay hoac 1MB. Anomaly ID la UUID duy nhat.", {
    x: 0.9, y: 3.8, w: 3.7, h: 1.0,
    fontSize: 11, fontFace: FONT_B, color: C.textDim,
  });

  card(s, 5.2, 3.3, 4.2, 1.8, C.card);
  s.addShape(pres.shapes.RECTANGLE, {
    x: 5.2, y: 3.3, w: 0.06, h: 1.8, fill: { color: C.teal },
  });
  s.addText("Hanh dong 2: Canh bao", {
    x: 5.5, y: 3.4, w: 3.5, h: 0.35,
    fontSize: 13, fontFace: FONT_H, color: C.teal, bold: true,
  });
  s.addText("AlertChannel (hien chua ket noi). Giao dien san de tich hop: Slack, Email, PagerDuty. Dispatcher chiu loi — loi duoc log, khong gian doan pipeline.", {
    x: 5.5, y: 3.8, w: 3.7, h: 1.0,
    fontSize: 11, fontFace: FONT_B, color: C.textDim,
  });
}

// ============================================================================
// SLIDE 15: Sliding Window
// ============================================================================
{
  const s = pres.addSlide();
  darkSlide(s); addTopBar(s); addSlideNumber(s, 15, TOTAL);
  addSectionTitle(s, "05 — Thuat toan phat hien bat thuong");

  s.addText("Cua so truot (Sliding Window)", {
    x: 0.6, y: 0.5, w: 9, h: 0.6,
    fontSize: 28, fontFace: FONT_H, color: C.white, bold: true, margin: 0,
  });

  // Circular buffer visualization
  card(s, 0.6, 1.3, 5.5, 2.5, C.card);
  s.addText("Circular Buffer — Capacity 10,000", {
    x: 0.9, y: 1.4, w: 5, h: 0.35,
    fontSize: 14, fontFace: FONT_H, color: C.accent, bold: true,
  });

  // Buffer cells
  const cells = [
    ["10:03", true], ["09:58", false], ["10:05", true], ["10:01", true], ["09:55", false], ["10:04", true], ["head\n\u2192", null],
  ];
  cells.forEach(([val, inWindow], i) => {
    const xx = 0.9 + i * 0.72;
    const color = inWindow === null ? C.gray : inWindow ? C.teal : C.grayDark;
    s.addShape(pres.shapes.RECTANGLE, {
      x: xx, y: 2.0, w: 0.65, h: 0.65, fill: { color: inWindow === null ? C.dark : C.card },
      line: { color, width: 1.5 },
    });
    s.addText(val, {
      x: xx, y: 2.0, w: 0.65, h: 0.65,
      fontSize: 9, fontFace: inWindow === null ? FONT_H : "Consolas", color: inWindow === null ? C.gray : C.white,
      align: "center", valign: "middle", bold: inWindow === null,
    });
  });

  s.addText([
    { text: "\u2588 Trong window (< 5 phut)    ", options: { color: C.teal, fontSize: 9 } },
    { text: "\u2588 Ngoai window", options: { color: C.grayDark, fontSize: 9 } },
  ], { x: 0.9, y: 2.8, w: 5, h: 0.25, fontFace: FONT_B });

  s.addText("Timestamp KHONG tang dan — Kafka 3 partitions\ngui song song nen phai scan TOAN BO buffer.", {
    x: 0.9, y: 3.1, w: 4.8, h: 0.5,
    fontSize: 10, fontFace: FONT_B, color: C.yellow, italic: true,
  });

  // Three operations
  card(s, 6.4, 1.3, 3.0, 2.5, C.card);
  s.addText("3 thao tac", {
    x: 6.7, y: 1.4, w: 2.5, h: 0.35,
    fontSize: 14, fontFace: FONT_H, color: C.blue, bold: true,
  });

  const ops = [
    [C.teal, "Add(t)", "O(1)", "Ghi tai head % cap"],
    [C.accent, "CountWithin(d)", "O(n)", "Scan all, dem > cutoff"],
    [C.yellow, "Evict(d)", "O(n)", "Compact, moi 30s"],
  ];
  ops.forEach(([color, name, complexity, desc], i) => {
    const yy = 1.85 + i * 0.62;
    s.addText(name, {
      x: 6.7, y: yy, w: 2.5, h: 0.25,
      fontSize: 11, fontFace: "Consolas", color, bold: true,
    });
    s.addText(`${complexity} — ${desc}`, {
      x: 6.7, y: yy + 0.23, w: 2.5, h: 0.25,
      fontSize: 9, fontFace: FONT_B, color: C.textDim,
    });
  });

  // Effective window warning
  card(s, 0.6, 4.1, 8.8, 1.1, C.card);
  s.addShape(pres.shapes.RECTANGLE, {
    x: 0.6, y: 4.1, w: 8.8, h: 0.06, fill: { color: C.red },
  });
  s.addText("Buffer day \u2192 Effective window bi rut ngan", {
    x: 0.9, y: 4.2, w: 5, h: 0.3,
    fontSize: 12, fontFace: FONT_H, color: C.red, bold: true,
  });
  s.addText("Capacity 10,000 / 300s window = toi da 33 msg/s/service. Rate cao hon → entry moi ghi de entry cu → window thuc te < 5 phut.", {
    x: 0.9, y: 4.55, w: 8.3, h: 0.5,
    fontSize: 11, fontFace: FONT_B, color: C.textDim,
  });
}

// ============================================================================
// SLIDE 16: Rules 1 & 2
// ============================================================================
{
  const s = pres.addSlide();
  darkSlide(s); addTopBar(s); addSlideNumber(s, 16, TOTAL);
  addSectionTitle(s, "05 — Thuat toan phat hien bat thuong");

  s.addText("Quy tac phat hien 1 & 2", {
    x: 0.6, y: 0.5, w: 9, h: 0.6,
    fontSize: 28, fontFace: FONT_H, color: C.white, bold: true, margin: 0,
  });

  // Rule 1
  card(s, 0.6, 1.3, 4.2, 3.5, C.card);
  s.addShape(pres.shapes.RECTANGLE, {
    x: 0.6, y: 1.3, w: 4.2, h: 0.06, fill: { color: C.red },
  });
  s.addText("Error Rate Spike", {
    x: 0.9, y: 1.45, w: 3.5, h: 0.35,
    fontSize: 16, fontFace: FONT_H, color: C.red, bold: true,
  });
  s.addText("Phat hien tang dot bien ty le loi cua mot dich vu", {
    x: 0.9, y: 1.8, w: 3.5, h: 0.3,
    fontSize: 10, fontFace: FONT_B, color: C.gray, italic: true,
  });

  const r1conf = [
    ["Nguong", "\u2265 10 errors"],
    ["Window", "5 phut"],
    ["Severity", "HIGH"],
    ["Theo doi", "Per service"],
  ];
  r1conf.forEach(([k, v], i) => {
    const yy = 2.3 + i * 0.35;
    s.addText(k, { x: 0.9, y: yy, w: 1.5, h: 0.3, fontSize: 10, fontFace: FONT_B, color: C.gray, valign: "middle" });
    s.addText(v, { x: 2.4, y: yy, w: 2.2, h: 0.3, fontSize: 11, fontFace: FONT_H, color: C.white, bold: true, valign: "middle" });
  });

  s.addText("Hash map: service \u2192 sliding window.\nLog error den \u2192 Add timestamp \u2192 CountWithin(5m) \u2192 \u2265 10 \u2192 FIRE", {
    x: 0.9, y: 3.8, w: 3.7, h: 0.8,
    fontSize: 10, fontFace: FONT_B, color: C.textDim,
  });

  // Rule 2
  card(s, 5.2, 1.3, 4.2, 3.5, C.card);
  s.addShape(pres.shapes.RECTANGLE, {
    x: 5.2, y: 1.3, w: 4.2, h: 0.06, fill: { color: C.yellow },
  });
  s.addText("Latency Threshold Breach", {
    x: 5.5, y: 1.45, w: 3.5, h: 0.35,
    fontSize: 16, fontFace: FONT_H, color: C.yellow, bold: true,
  });
  s.addText("Phat hien ty le yeu cau co do tre vuot nguong", {
    x: 5.5, y: 1.8, w: 3.5, h: 0.3,
    fontSize: 10, fontFace: FONT_B, color: C.gray, italic: true,
  });

  const r2conf = [
    ["Nguong", "\u2265 500ms"],
    ["Breach rate", "\u2265 20%"],
    ["Window", "5 phut"],
    ["Severity", "MEDIUM"],
  ];
  r2conf.forEach(([k, v], i) => {
    const yy = 2.3 + i * 0.35;
    s.addText(k, { x: 5.5, y: yy, w: 1.5, h: 0.3, fontSize: 10, fontFace: FONT_B, color: C.gray, valign: "middle" });
    s.addText(v, { x: 7.0, y: yy, w: 2.2, h: 0.3, fontSize: 11, fontFace: FONT_H, color: C.white, bold: true, valign: "middle" });
  });

  s.addText("Latency window hai chieu: (timestamp, breached).\nbreach_rate = (breaches \u00D7 100) / total \u2265 20% \u2192 FIRE", {
    x: 5.5, y: 3.8, w: 3.7, h: 0.8,
    fontSize: 10, fontFace: FONT_B, color: C.textDim,
  });
}

// ============================================================================
// SLIDE 17: Rules 3 & 4
// ============================================================================
{
  const s = pres.addSlide();
  darkSlide(s); addTopBar(s); addSlideNumber(s, 17, TOTAL);
  addSectionTitle(s, "05 — Thuat toan phat hien bat thuong");

  s.addText("Quy tac phat hien 3 & 4", {
    x: 0.6, y: 0.5, w: 9, h: 0.6,
    fontSize: 28, fontFace: FONT_H, color: C.white, bold: true, margin: 0,
  });

  // Rule 3
  card(s, 0.6, 1.3, 4.2, 3.5, C.card);
  s.addShape(pres.shapes.RECTANGLE, {
    x: 0.6, y: 1.3, w: 4.2, h: 0.06, fill: { color: C.accent },
  });
  s.addText("Repeated Failure", {
    x: 0.9, y: 1.45, w: 3.5, h: 0.35,
    fontSize: 16, fontFace: FONT_H, color: C.accent, bold: true,
  });
  s.addText("Loi lap lai cung mau — van de chua duoc xu ly", {
    x: 0.9, y: 1.8, w: 3.5, h: 0.3,
    fontSize: 10, fontFace: FONT_B, color: C.gray, italic: true,
  });

  s.addText([
    { text: "Message Fingerprinting:", options: { bold: true, breakLine: true, color: C.white } },
    { text: "So + hex \u2192 \"X\"", options: { breakLine: true, color: C.textDim } },
    { text: "\"timeout req abc1234 after 5000ms\"", options: { breakLine: true, color: C.gray, italic: true } },
    { text: "\u2192 \"timeout req X after Xms\"", options: { breakLine: true, color: C.accent } },
    { text: "", options: { breakLine: true } },
    { text: "Khoa: service|fingerprint", options: { breakLine: true, color: C.textDim } },
    { text: "Nguong: \u2265 5 lan trong 5 phut", options: { breakLine: true, color: C.textDim } },
    { text: "Severity: MEDIUM", options: { color: C.textDim } },
  ], {
    x: 0.9, y: 2.2, w: 3.7, h: 2.4,
    fontSize: 11, fontFace: FONT_B, color: C.textDim, paraSpaceAfter: 2,
  });

  // Rule 4
  card(s, 5.2, 1.3, 4.2, 3.5, C.card);
  s.addShape(pres.shapes.RECTANGLE, {
    x: 5.2, y: 1.3, w: 4.2, h: 0.06, fill: { color: C.blue },
  });
  s.addText("Auth Failure Burst", {
    x: 5.5, y: 1.45, w: 3.5, h: 0.35,
    fontSize: 16, fontFace: FONT_H, color: C.blue, bold: true,
  });
  s.addText("Phat hien tan cong brute-force xac thuc", {
    x: 5.5, y: 1.8, w: 3.5, h: 0.3,
    fontSize: 10, fontFace: FONT_B, color: C.gray, italic: true,
  });

  s.addText([
    { text: "Nhan dien auth failure:", options: { bold: true, breakLine: true, color: C.white } },
    { text: "Tu khoa: auth, login, unauthorized", options: { breakLine: true, color: C.textDim } },
    { text: "Hoac: auth_result = \"failure\"", options: { breakLine: true, color: C.textDim } },
    { text: "", options: { breakLine: true } },
    { text: "Theo doi: source_ip (uu tien) > username", options: { breakLine: true, color: C.textDim } },
    { text: "Nguong: \u2265 10 failures / 5 phut / IP", options: { breakLine: true, color: C.textDim } },
    { text: "Severity: HIGH", options: { color: C.textDim } },
  ], {
    x: 5.5, y: 2.2, w: 3.7, h: 2.4,
    fontSize: 11, fontFace: FONT_B, color: C.textDim, paraSpaceAfter: 2,
  });
}

// ============================================================================
// SLIDE 18: Rules 5 & 6
// ============================================================================
{
  const s = pres.addSlide();
  darkSlide(s); addTopBar(s); addSlideNumber(s, 18, TOTAL);
  addSectionTitle(s, "05 — Thuat toan phat hien bat thuong");

  s.addText("Quy tac phat hien 5 & 6", {
    x: 0.6, y: 0.5, w: 9, h: 0.6,
    fontSize: 28, fontFace: FONT_H, color: C.white, bold: true, margin: 0,
  });

  // Rule 5
  card(s, 0.6, 1.3, 4.2, 3.5, C.card);
  s.addShape(pres.shapes.RECTANGLE, {
    x: 0.6, y: 1.3, w: 4.2, h: 0.06, fill: { color: C.teal },
  });
  s.addText("Off-Hours Access", {
    x: 0.9, y: 1.45, w: 3.5, h: 0.35,
    fontSize: 16, fontFace: FONT_H, color: C.teal, bold: true,
  });
  s.addText("Truy cap duong dan nhay cam ngoai gio lam viec", {
    x: 0.9, y: 1.8, w: 3.5, h: 0.3,
    fontSize: 10, fontFace: FONT_B, color: C.gray, italic: true,
  });
  s.addText([
    { text: "Stateless — khong dung sliding window", options: { bold: true, breakLine: true, color: C.yellow } },
    { text: "", options: { breakLine: true } },
    { text: "Dieu kien 1: path thuoc danh sach nhay cam", options: { breakLine: true, color: C.textDim } },
    { text: "  /admin, /api/v1/users, /internal", options: { breakLine: true, color: C.gray, italic: true } },
    { text: "Dieu kien 2: ngoai gio 9h-17h UTC", options: { breakLine: true, color: C.textDim } },
    { text: "", options: { breakLine: true } },
    { text: "Ca hai thoa man \u2192 FIRE ngay lap tuc", options: { breakLine: true, color: C.teal } },
    { text: "Severity: MEDIUM | O(p), O(1) space", options: { color: C.gray } },
  ], {
    x: 0.9, y: 2.2, w: 3.7, h: 2.4,
    fontSize: 11, fontFace: FONT_B, color: C.textDim, paraSpaceAfter: 2,
  });

  // Rule 6
  card(s, 5.2, 1.3, 4.2, 3.5, C.card);
  s.addShape(pres.shapes.RECTANGLE, {
    x: 5.2, y: 1.3, w: 4.2, h: 0.06, fill: { color: C.red },
  });
  s.addText("Service Silence", {
    x: 5.5, y: 1.45, w: 3.5, h: 0.35,
    fontSize: 16, fontFace: FONT_H, color: C.red, bold: true,
  });
  s.addText("Dich vu ngung gui log — co the da gap su co", {
    x: 5.5, y: 1.8, w: 3.5, h: 0.3,
    fontSize: 10, fontFace: FONT_B, color: C.gray, italic: true,
  });
  s.addText([
    { text: "Mo hinh 2 pha:", options: { bold: true, breakLine: true, color: C.yellow } },
    { text: "", options: { breakLine: true } },
    { text: "Pha 1 (Evaluate): dem log/service", options: { breakLine: true, color: C.textDim } },
    { text: "Chi bat dau theo doi khi \u2265 5 log (cold-start guard)", options: { breakLine: true, color: C.gray, italic: true } },
    { text: "", options: { breakLine: true } },
    { text: "Pha 2 (CheckSilence): heartbeat 30s", options: { breakLine: true, color: C.textDim } },
    { text: "Im lang > 5 phut \u2192 FIRE", options: { breakLine: true, color: C.red } },
    { text: "Severity: CRITICAL | sync.Mutex", options: { color: C.gray } },
  ], {
    x: 5.5, y: 2.2, w: 3.7, h: 2.4,
    fontSize: 11, fontFace: FONT_B, color: C.textDim, paraSpaceAfter: 2,
  });
}

// ============================================================================
// SLIDE 19: Warmup, Cooldown, Eviction
// ============================================================================
{
  const s = pres.addSlide();
  darkSlide(s); addTopBar(s); addSlideNumber(s, 19, TOTAL);
  addSectionTitle(s, "05 — Thuat toan phat hien bat thuong");

  s.addText("Warmup, Cooldown & Eviction", {
    x: 0.6, y: 0.5, w: 9, h: 0.6,
    fontSize: 28, fontFace: FONT_H, color: C.white, bold: true, margin: 0,
  });

  const mechs = [
    [C.accent, "Warmup", "10 phut sau khoi dong", "Suppress TAT CA anomaly de cua so truot tich luy du du lieu. Cong thuc: multiplier(2) \u00D7 max_window(5m) = 10m. Chay 1 lan duy nhat.", "Khi nao: App khoi dong\nSuppress: Tat ca rule, tat ca service"],
    [C.blue, "Cooldown", "15 phut sau moi FIRE", "Suppress cung cap (rule_id, service) de tranh alert storm. Error spike tren payment-service fire → 15 phut tiep suppress. Rule khac hoac service khac KHONG bi anh huong.", "Khi nao: Sau moi anomaly\nSuppress: Chi (rule, service) cu the"],
    [C.teal, "Eviction", "Moi 30 giay", "Goroutine nen goi Reset() tren tat ca detector de don dep du lieu cu. Xoa cooldown entry het han (> 2x cooldown). Dam bao bo nho khong tang vo han.", "Khi nao: Dinh ky 30s\nNhiem vu: Don dep memory"],
  ];

  mechs.forEach(([color, title, timing, desc, scope], i) => {
    const yy = 1.2 + i * 1.4;
    card(s, 0.6, yy, 8.8, 1.2, C.card);
    s.addShape(pres.shapes.RECTANGLE, {
      x: 0.6, y: yy, w: 0.06, h: 1.2, fill: { color },
    });
    s.addText(title, {
      x: 0.9, y: yy + 0.05, w: 1.5, h: 0.3,
      fontSize: 16, fontFace: FONT_H, color, bold: true,
    });
    s.addText(timing, {
      x: 2.5, y: yy + 0.05, w: 2.5, h: 0.3,
      fontSize: 11, fontFace: FONT_B, color: C.gray, italic: true,
    });
    s.addText(desc, {
      x: 0.9, y: yy + 0.4, w: 5.5, h: 0.7,
      fontSize: 10, fontFace: FONT_B, color: C.textDim,
    });
    s.addText(scope, {
      x: 6.5, y: yy + 0.15, w: 2.7, h: 0.9,
      fontSize: 10, fontFace: FONT_B, color: C.white,
    });
  });
}

// ============================================================================
// SLIDE 20: Toi uu hieu nang — Xu ly song song
// ============================================================================
{
  const s = pres.addSlide();
  darkSlide(s); addTopBar(s); addSlideNumber(s, 20, TOTAL);
  addSectionTitle(s, "06 — Toi uu hieu nang");

  s.addText("Xu ly song song da cap", {
    x: 0.6, y: 0.5, w: 9, h: 0.6,
    fontSize: 28, fontFace: FONT_H, color: C.white, bold: true, margin: 0,
  });

  const levels = [
    [C.accent, "Kafka", "3 partitions\ndoc dong thoi"],
    [C.blue, "Pipeline", "4 goroutine workers\nfan-out tu channel"],
    [C.teal, "BulkIndexer", "4 worker threads\nbulk request song song"],
    [C.green, "Detection", "In-memory evaluate\nkhong I/O blocking"],
  ];

  levels.forEach(([color, title, desc], i) => {
    const xx = 0.4 + i * 2.4;
    s.addShape(pres.shapes.RECTANGLE, {
      x: xx, y: 1.4, w: 2.1, h: 1.8, fill: { color: C.card },
      shadow: shadow(), line: { color, width: 1.5 },
    });
    iconCircle(s, xx + 0.7, 1.6, 0.6, color, `${i + 1}`);
    s.addText(title, {
      x: xx, y: 2.3, w: 2.1, h: 0.35,
      fontSize: 14, fontFace: FONT_H, color, bold: true, align: "center",
    });
    s.addText(desc, {
      x: xx + 0.1, y: 2.65, w: 1.9, h: 0.5,
      fontSize: 10, fontFace: FONT_B, color: C.textDim, align: "center",
    });
  });

  // Channel buffers
  card(s, 0.6, 3.5, 8.8, 1.8, C.card);
  s.addText("Channel buffer — Bo giam xoc (shock absorber)", {
    x: 0.9, y: 3.6, w: 8, h: 0.35,
    fontSize: 14, fontFace: FONT_H, color: C.yellow, bold: true,
  });

  s.addText([
    { text: "Consumer channel (10,000 msg): ", options: { bold: true, color: C.white } },
    { text: "Kafka doc nhanh, worker cham → channel hap thu burst. Day → backpressure tu nhien.", options: { color: C.textDim, breakLine: true } },
    { text: "Anomaly channel (1,024 msg): ", options: { bold: true, color: C.white } },
    { text: "Detection nhanh, dispatcher ghi ES cham → buffer hap thu. Day → drop voi warning (chap nhan mat anomaly hon lam cham pipeline).", options: { color: C.textDim } },
  ], {
    x: 0.9, y: 4.05, w: 8.3, h: 1.0,
    fontSize: 11, fontFace: FONT_B, paraSpaceAfter: 6,
  });
}

// ============================================================================
// SLIDE 21: Bulk Indexing & Idempotent
// ============================================================================
{
  const s = pres.addSlide();
  darkSlide(s); addTopBar(s); addSlideNumber(s, 21, TOTAL);
  addSectionTitle(s, "06 — Toi uu hieu nang");

  s.addText("Ghi theo lo & Xu ly idempotent", {
    x: 0.6, y: 0.5, w: 9, h: 0.6,
    fontSize: 28, fontFace: FONT_H, color: C.white, bold: true, margin: 0,
  });

  // Bulk indexing
  card(s, 0.6, 1.3, 4.2, 2.2, C.card);
  s.addShape(pres.shapes.RECTANGLE, {
    x: 0.6, y: 1.3, w: 0.06, h: 2.2, fill: { color: C.blue },
  });
  s.addText("BulkIndexer", {
    x: 0.9, y: 1.4, w: 3.5, h: 0.35,
    fontSize: 16, fontFace: FONT_H, color: C.blue, bold: true,
  });
  s.addText([
    { text: "Gom yeu cau ghi thanh lo (batch)", options: { bullet: true, breakLine: true } },
    { text: "Flush khi dat 10MB hoac sau 5 giay", options: { bullet: true, breakLine: true } },
    { text: "4 worker threads gui bulk song song", options: { bullet: true, breakLine: true } },
    { text: "Giam so round-trip HTTP hang tram lan", options: { bullet: true } },
  ], {
    x: 0.9, y: 1.9, w: 3.7, h: 1.3,
    fontSize: 11, fontFace: FONT_B, color: C.textDim, paraSpaceAfter: 6,
  });

  // Idempotent
  card(s, 5.2, 1.3, 4.2, 2.2, C.card);
  s.addShape(pres.shapes.RECTANGLE, {
    x: 5.2, y: 1.3, w: 0.06, h: 2.2, fill: { color: C.teal },
  });
  s.addText("SHA-256 Document ID", {
    x: 5.5, y: 1.4, w: 3.5, h: 0.35,
    fontSize: 16, fontFace: FONT_H, color: C.teal, bold: true,
  });
  s.addText([
    { text: "ID = SHA-256(\"partition:offset\")", options: { bullet: true, breakLine: true } },
    { text: "Cung message → cung document ID", options: { bullet: true, breakLine: true } },
    { text: "Xu ly lai → ghi de, khong trung lap", options: { bullet: true, breakLine: true } },
    { text: "Phu hop at-least-once cua Kafka", options: { bullet: true } },
  ], {
    x: 5.5, y: 1.9, w: 3.7, h: 1.3,
    fontSize: 11, fontFace: FONT_B, color: C.textDim, paraSpaceAfter: 6,
  });

  // Memory optimization
  card(s, 0.6, 3.8, 8.8, 1.5, C.card);
  s.addText("Toi uu bo nho", {
    x: 0.9, y: 3.9, w: 3, h: 0.35,
    fontSize: 14, fontFace: FONT_H, color: C.accent, bold: true,
  });
  s.addText("Circular buffer 10,000 entries co dinh → O(1) memory bat ke thoi gian chay. Khong cap phat/giai phong → khong ap luc GC. Hash map voi don dep dinh ky → khong tang truong vo han. Refresh interval 5s (thay vi 1s) → giam segment Lucene x5.", {
    x: 0.9, y: 4.3, w: 8.3, h: 0.8,
    fontSize: 11, fontFace: FONT_B, color: C.textDim,
  });
}

// ============================================================================
// SLIDE 22: Prometheus Metrics
// ============================================================================
{
  const s = pres.addSlide();
  darkSlide(s); addTopBar(s); addSlideNumber(s, 22, TOTAL);
  addSectionTitle(s, "07 — He thong giam sat");

  s.addText("Prometheus Metrics", {
    x: 0.6, y: 0.5, w: 9, h: 0.6,
    fontSize: 28, fontFace: FONT_H, color: C.white, bold: true, margin: 0,
  });

  const metrics = [
    [{ text: "logs_consumed_total", options: { bold: true, color: C.white, fill: { color: C.midBlue } } },
     { text: "Loai", options: { bold: true, color: C.white, fill: { color: C.midBlue } } },
     { text: "Mo ta", options: { bold: true, color: C.white, fill: { color: C.midBlue } } }],
    ["logs_consumed_total", "counter", "Tong log da doc tu Kafka (topic, partition)"],
    ["logs_processed_duration_seconds", "histogram", "Do tre xu ly e2e (p50, p95, p99)"],
    ["anomalies_detected_total", "counter", "So anomaly phat hien (theo rule)"],
    ["kafka_consumer_lag", "gauge", "Khoang cach offset (topic, partition)"],
    ["elasticsearch_write_errors_total", "counter", "So loi ghi Elasticsearch"],
    ["parse_errors_total", "counter", "So log khong the parse JSON"],
  ];

  s.addTable(metrics, {
    x: 0.6, y: 1.3, w: 8.8, colW: [3.2, 1.3, 4.3],
    fontSize: 11, fontFace: FONT_B, color: C.white,
    border: { pt: 0.5, color: C.cardBorder },
    fill: { color: C.card },
    altColor: C.darkBlue,
    autoPage: false,
  });

  s.addText("Endpoint: :2112/metrics | Scrape interval: 15 giay", {
    x: 0.6, y: 4.9, w: 8, h: 0.3,
    fontSize: 10, fontFace: "Consolas", color: C.gray,
  });
}

// ============================================================================
// SLIDE 23: Grafana & Kibana
// ============================================================================
{
  const s = pres.addSlide();
  darkSlide(s); addTopBar(s); addSlideNumber(s, 23, TOTAL);
  addSectionTitle(s, "07 — He thong giam sat");

  s.addText("Truc quan hoa — Grafana & Kibana", {
    x: 0.6, y: 0.5, w: 9, h: 0.6,
    fontSize: 28, fontFace: FONT_H, color: C.white, bold: true, margin: 0,
  });

  // Grafana
  card(s, 0.6, 1.3, 4.2, 3.5, C.card);
  s.addShape(pres.shapes.RECTANGLE, {
    x: 0.6, y: 1.3, w: 4.2, h: 0.06, fill: { color: C.accent },
  });
  s.addText("Grafana 12.0 — Dashboard", {
    x: 0.9, y: 1.45, w: 3.5, h: 0.35,
    fontSize: 14, fontFace: FONT_H, color: C.accent, bold: true,
  });

  s.addText("Log Analytics & Anomaly Detection:", {
    x: 0.9, y: 1.9, w: 3.5, h: 0.25,
    fontSize: 11, fontFace: FONT_H, color: C.white, bold: true,
  });
  s.addText([
    { text: "Ty le tiep nhan log theo thoi gian", options: { bullet: true, breakLine: true } },
    { text: "Do tre xu ly (p50/p95/p99)", options: { bullet: true, breakLine: true } },
    { text: "Anomaly rate theo quy tac", options: { bullet: true, breakLine: true } },
    { text: "Consumer lag, ES errors, parse errors", options: { bullet: true } },
  ], {
    x: 0.9, y: 2.2, w: 3.7, h: 1.3,
    fontSize: 10, fontFace: FONT_B, color: C.textDim, paraSpaceAfter: 4,
  });

  s.addText("Log Explorer:", {
    x: 0.9, y: 3.5, w: 3.5, h: 0.25,
    fontSize: 11, fontFace: FONT_H, color: C.white, bold: true,
  });
  s.addText("Luong log, phan bo service/level, bang anomaly chi tiet.", {
    x: 0.9, y: 3.75, w: 3.7, h: 0.6,
    fontSize: 10, fontFace: FONT_B, color: C.textDim,
  });

  // Kibana
  card(s, 5.2, 1.3, 4.2, 3.5, C.card);
  s.addShape(pres.shapes.RECTANGLE, {
    x: 5.2, y: 1.3, w: 4.2, h: 0.06, fill: { color: C.teal },
  });
  s.addText("Kibana 9.0 — Log Viewer", {
    x: 5.5, y: 1.45, w: 3.5, h: 0.35,
    fontSize: 14, fontFace: FONT_H, color: C.teal, bold: true,
  });

  s.addText([
    { text: "Trang Discover: xem tung dong log", options: { bullet: true, breakLine: true } },
    { text: "KQL filter: service, level, IP...", options: { bullet: true, breakLine: true } },
    { text: "Bieu do xu huong tich hop san", options: { bullet: true, breakLine: true } },
    { text: "", options: { breakLine: true } },
    { text: "2 Data View tu dong:", options: { bold: true, breakLine: true, color: C.white } },
    { text: "Application Logs (logs-*)", options: { bullet: true, breakLine: true } },
    { text: "Anomalies (anomalies)", options: { bullet: true, breakLine: true } },
    { text: "", options: { breakLine: true } },
    { text: "kibana-init container tao san khi\nhe thong khoi dong lan dau.", options: { color: C.gray, italic: true } },
  ], {
    x: 5.5, y: 1.9, w: 3.7, h: 2.7,
    fontSize: 10, fontFace: FONT_B, color: C.textDim, paraSpaceAfter: 2,
  });
}

// ============================================================================
// SLIDE 24: Kiem thu
// ============================================================================
{
  const s = pres.addSlide();
  darkSlide(s); addTopBar(s); addSlideNumber(s, 24, TOTAL);
  addSectionTitle(s, "08 — Kiem thu");

  s.addText("Kiem thu & Dam bao chat luong", {
    x: 0.6, y: 0.5, w: 9, h: 0.6,
    fontSize: 28, fontFace: FONT_H, color: C.white, bold: true, margin: 0,
  });

  // Three test types
  const tests = [
    [C.accent, "E2E Test", "cmd/e2etest", "5 buoc tu dong: health check \u2192 baseline \u2192 produce 6 pha \u2192 verify drain \u2192 report. Ket qua: PASS (\u226599%), WARN (95-99%), FAIL (<95%)."],
    [C.blue, "Load Test", "cmd/loadtest", "7 pha trong 5 phut, 4 producer workers. Mo phong tat ca loai anomaly. Rate cau hinh duoc (mac dinh 500 msg/s). Dung de kiem tra gioi han he thong."],
    [C.teal, "Unit + Integration", "go test", "Unit test voi testify + mock. Integration test voi testcontainers-go. 10,000 messages, kiem tra 5 rule, assert throughput \u2265 500 msg/s, parse error < 1%."],
  ];

  tests.forEach(([color, title, cmd, desc], i) => {
    const yy = 1.2 + i * 1.4;
    card(s, 0.6, yy, 8.8, 1.2, C.card);
    s.addShape(pres.shapes.RECTANGLE, {
      x: 0.6, y: yy, w: 0.06, h: 1.2, fill: { color },
    });
    s.addText(title, {
      x: 0.9, y: yy + 0.05, w: 2, h: 0.35,
      fontSize: 16, fontFace: FONT_H, color, bold: true,
    });
    s.addText(cmd, {
      x: 3.0, y: yy + 0.08, w: 2, h: 0.3,
      fontSize: 10, fontFace: "Consolas", color: C.gray,
    });
    s.addText(desc, {
      x: 0.9, y: yy + 0.45, w: 8.3, h: 0.65,
      fontSize: 11, fontFace: FONT_B, color: C.textDim,
    });
  });

  // Key results
  card(s, 0.6, 4.5, 8.8, 0.8, C.card);
  s.addShape(pres.shapes.RECTANGLE, {
    x: 0.6, y: 4.5, w: 8.8, h: 0.06, fill: { color: C.green },
  });

  const stats = [
    ["179,952", "messages sent"],
    ["100%", "indexed"],
    ["582", "msg/s throughput"],
    ["7", "anomalies detected"],
    ["9s", "drain time"],
  ];
  stats.forEach(([value, label], i) => {
    const xx = 0.8 + i * 1.8;
    s.addText(value, {
      x: xx, y: 4.55, w: 1.5, h: 0.35,
      fontSize: 18, fontFace: FONT_H, color: C.green, bold: true, align: "center",
    });
    s.addText(label, {
      x: xx, y: 4.85, w: 1.5, h: 0.25,
      fontSize: 9, fontFace: FONT_B, color: C.gray, align: "center",
    });
  });
}

// ============================================================================
// SLIDE 25: Ket luan
// ============================================================================
{
  const s = pres.addSlide();
  s.background = { color: C.dark };
  s.addShape(pres.shapes.RECTANGLE, {
    x: 0, y: 0, w: 10, h: 5.625,
    fill: { color: C.darkBlue, transparency: 30 },
  });
  s.addShape(pres.shapes.RECTANGLE, {
    x: 0, y: 5.0, w: 10, h: 0.625, fill: { color: C.blue, transparency: 60 },
  });
  addSlideNumber(s, 25, TOTAL);

  s.addText("Ket luan & Huong phat trien", {
    x: 0.6, y: 0.4, w: 8, h: 0.6,
    fontSize: 32, fontFace: FONT_H, color: C.white, bold: true, margin: 0,
  });

  // Summary
  card(s, 0.6, 1.2, 4.2, 3.5, C.card);
  s.addShape(pres.shapes.RECTANGLE, {
    x: 0.6, y: 1.2, w: 0.06, h: 3.5, fill: { color: C.teal },
  });
  s.addText("Da dat duoc", {
    x: 0.9, y: 1.3, w: 3.5, h: 0.35,
    fontSize: 16, fontFace: FONT_H, color: C.teal, bold: true,
  });
  s.addText([
    { text: "Kien truc event-driven voi Kafka + Go pipeline", options: { bullet: true, breakLine: true } },
    { text: "6 quy tac phat hien bat thuong sliding window", options: { bullet: true, breakLine: true } },
    { text: "Xu ly song song da cap (Kafka, Worker, ES)", options: { bullet: true, breakLine: true } },
    { text: "Idempotent processing (SHA-256 doc ID)", options: { bullet: true, breakLine: true } },
    { text: "Warmup + Cooldown + Eviction bao ve", options: { bullet: true, breakLine: true } },
    { text: "Bo quan sat: Kibana + Grafana + Prometheus", options: { bullet: true, breakLine: true } },
    { text: "E2E test PASS: 180K msgs, 582 msg/s, 100%", options: { bullet: true } },
  ], {
    x: 0.9, y: 1.75, w: 3.7, h: 2.7,
    fontSize: 11, fontFace: FONT_B, color: C.textDim, paraSpaceAfter: 4,
  });

  // Future
  card(s, 5.2, 1.2, 4.2, 3.5, C.card);
  s.addShape(pres.shapes.RECTANGLE, {
    x: 5.2, y: 1.2, w: 0.06, h: 3.5, fill: { color: C.accent },
  });
  s.addText("Huong phat trien", {
    x: 5.5, y: 1.3, w: 3.5, h: 0.35,
    fontSize: 16, fontFace: FONT_H, color: C.accent, bold: true,
  });
  s.addText([
    { text: "Tich hop AlertChannel: Slack, PagerDuty", options: { bullet: true, breakLine: true } },
    { text: "Quy tac phat hien dua tren Machine Learning", options: { bullet: true, breakLine: true } },
    { text: "Horizontal scaling nhieu consumer instance", options: { bullet: true, breakLine: true } },
    { text: "Retention tu dong cho index ES cu", options: { bullet: true, breakLine: true } },
    { text: "Dashboard Grafana nang cao hon", options: { bullet: true } },
  ], {
    x: 5.5, y: 1.75, w: 3.7, h: 2.7,
    fontSize: 11, fontFace: FONT_B, color: C.textDim, paraSpaceAfter: 6,
  });

  s.addText("Production-Ready | Event-Driven | Real-time Detection", {
    x: 0.6, y: 5.1, w: 8, h: 0.35,
    fontSize: 13, fontFace: FONT_H, color: C.white, bold: true,
  });
}

// ── Generate ────────────────────────────────────────────────────────────────

const outPath = process.argv[2] || "docs/presentation.pptx";
pres.writeFile({ fileName: outPath }).then(() => {
  console.log(`Generated: ${outPath} (${TOTAL} slides)`);
}).catch(err => {
  console.error("Error:", err);
  process.exit(1);
});
