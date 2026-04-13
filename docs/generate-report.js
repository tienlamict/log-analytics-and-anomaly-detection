const fs = require("fs");
const {
  Document, Packer, Paragraph, TextRun, Table, TableRow, TableCell,
  Header, Footer, AlignmentType, HeadingLevel, BorderStyle, WidthType,
  ShadingType, PageNumber, PageBreak, LevelFormat, TabStopType, TabStopPosition,
} = require("docx");

// ── Helpers ──────────────────────────────────────────────────────────────────

const PAGE_W = 12240;
const PAGE_H = 15840;
const MARGIN = 1440;
const CONTENT_W = PAGE_W - 2 * MARGIN; // 9360

const FONT = "Times New Roman";
const BLUE = "1F4E79";
const GRAY = "F2F2F2";
const BORDER = { style: BorderStyle.SINGLE, size: 1, color: "BBBBBB" };
const BORDERS = { top: BORDER, bottom: BORDER, left: BORDER, right: BORDER };

function p(text, opts = {}) {
  const runs = [];
  if (typeof text === "string") {
    // Parse **bold** markers
    const parts = text.split(/(\*\*[^*]+\*\*)/g);
    for (const part of parts) {
      if (part.startsWith("**") && part.endsWith("**")) {
        runs.push(new TextRun({ text: part.slice(2, -2), bold: true, font: FONT, size: 24 }));
      } else {
        runs.push(new TextRun({ text: part, font: FONT, size: 24, ...opts.runOpts }));
      }
    }
  } else {
    runs.push(...text);
  }
  return new Paragraph({
    spacing: { after: 200, line: 360 },
    alignment: AlignmentType.JUSTIFIED,
    ...opts,
    children: runs,
  });
}

function heading1(text) {
  return new Paragraph({
    heading: HeadingLevel.HEADING_1,
    spacing: { before: 480, after: 240 },
    children: [new TextRun({ text, font: FONT, size: 32, bold: true, color: BLUE })],
  });
}

function heading2(text) {
  return new Paragraph({
    heading: HeadingLevel.HEADING_2,
    spacing: { before: 360, after: 200 },
    children: [new TextRun({ text, font: FONT, size: 28, bold: true, color: BLUE })],
  });
}

function heading3(text) {
  return new Paragraph({
    heading: HeadingLevel.HEADING_3,
    spacing: { before: 240, after: 160 },
    children: [new TextRun({ text, font: FONT, size: 26, bold: true })],
  });
}

function cell(text, opts = {}) {
  return new TableCell({
    borders: BORDERS,
    width: { size: opts.width || 2340, type: WidthType.DXA },
    shading: opts.shading ? { fill: opts.shading, type: ShadingType.CLEAR } : undefined,
    margins: { top: 60, bottom: 60, left: 100, right: 100 },
    verticalAlign: "center",
    children: [
      new Paragraph({
        spacing: { after: 0, line: 276 },
        alignment: opts.align || AlignmentType.LEFT,
        children: [new TextRun({ text, font: FONT, size: 20, bold: !!opts.bold, color: opts.color })],
      }),
    ],
  });
}

function table(headers, rows, colWidths) {
  const totalW = colWidths.reduce((a, b) => a + b, 0);
  return new Table({
    width: { size: totalW, type: WidthType.DXA },
    columnWidths: colWidths,
    rows: [
      new TableRow({
        children: headers.map((h, i) =>
          cell(h, { width: colWidths[i], shading: BLUE, bold: true, color: "FFFFFF" })
        ),
      }),
      ...rows.map(
        (row, ri) =>
          new TableRow({
            children: row.map((c, i) =>
              cell(c, { width: colWidths[i], shading: ri % 2 === 0 ? GRAY : undefined })
            ),
          })
      ),
    ],
  });
}

function spacer() {
  return new Paragraph({ spacing: { after: 80 } });
}

// ── Document Content ─────────────────────────────────────────────────────────

const doc = new Document({
  styles: {
    default: {
      document: { run: { font: FONT, size: 24 } },
    },
    paragraphStyles: [
      {
        id: "Heading1", name: "Heading 1", basedOn: "Normal", next: "Normal", quickFormat: true,
        run: { size: 32, bold: true, font: FONT, color: BLUE },
        paragraph: { spacing: { before: 480, after: 240 }, outlineLevel: 0 },
      },
      {
        id: "Heading2", name: "Heading 2", basedOn: "Normal", next: "Normal", quickFormat: true,
        run: { size: 28, bold: true, font: FONT, color: BLUE },
        paragraph: { spacing: { before: 360, after: 200 }, outlineLevel: 1 },
      },
      {
        id: "Heading3", name: "Heading 3", basedOn: "Normal", next: "Normal", quickFormat: true,
        run: { size: 26, bold: true, font: FONT },
        paragraph: { spacing: { before: 240, after: 160 }, outlineLevel: 2 },
      },
    ],
  },
  numbering: {
    config: [
      {
        reference: "bullets",
        levels: [{
          level: 0, format: LevelFormat.BULLET, text: "\u2022", alignment: AlignmentType.LEFT,
          style: { paragraph: { indent: { left: 720, hanging: 360 } } },
        }],
      },
    ],
  },
  sections: [
    // ── TITLE PAGE ────────────────────────────────────────────────────────
    {
      properties: {
        page: {
          size: { width: PAGE_W, height: PAGE_H },
          margin: { top: MARGIN, right: MARGIN, bottom: MARGIN, left: MARGIN },
        },
      },
      children: [
        new Paragraph({ spacing: { before: 3000 } }),
        new Paragraph({
          alignment: AlignmentType.CENTER,
          spacing: { after: 200 },
          children: [new TextRun({ text: "BAO CAO KY THUAT", font: FONT, size: 36, bold: true, color: BLUE })],
        }),
        new Paragraph({ alignment: AlignmentType.CENTER, spacing: { after: 0 },
          children: [new TextRun({ text: "_______________", font: FONT, size: 28, color: BLUE })] }),
        new Paragraph({ spacing: { after: 400 } }),
        new Paragraph({
          alignment: AlignmentType.CENTER,
          spacing: { after: 600 },
          children: [new TextRun({ text: "HE THONG PHAN TICH LOG VA PHAT HIEN BAT THUONG\nTHOI GIAN THUC", font: FONT, size: 44, bold: true, color: BLUE })],
        }),
        new Paragraph({
          alignment: AlignmentType.CENTER,
          spacing: { after: 100 },
          children: [new TextRun({ text: "Real-time Log Analytics and Anomaly Detection System", font: FONT, size: 26, italics: true, color: "555555" })],
        }),
        new Paragraph({ spacing: { before: 1200 } }),
        new Paragraph({
          alignment: AlignmentType.CENTER,
          spacing: { after: 100 },
          children: [new TextRun({ text: "Cong nghe: Go, Apache Kafka, Elasticsearch, Kibana, Grafana, Prometheus", font: FONT, size: 22, color: "666666" })],
        }),
        new Paragraph({
          alignment: AlignmentType.CENTER,
          spacing: { after: 100 },
          children: [new TextRun({ text: "Kien truc: Event-Driven Pipeline, Sliding Window Detection", font: FONT, size: 22, color: "666666" })],
        }),
        new Paragraph({ spacing: { before: 800 } }),
        new Paragraph({
          alignment: AlignmentType.CENTER,
          children: [new TextRun({ text: "Thang 4/2026", font: FONT, size: 24, color: "888888" })],
        }),
      ],
    },

    // ── MAIN CONTENT ──────────────────────────────────────────────────────
    {
      properties: {
        page: {
          size: { width: PAGE_W, height: PAGE_H },
          margin: { top: MARGIN, right: MARGIN, bottom: MARGIN, left: MARGIN },
        },
      },
      headers: {
        default: new Header({
          children: [
            new Paragraph({
              alignment: AlignmentType.RIGHT,
              border: { bottom: { style: BorderStyle.SINGLE, size: 6, color: BLUE, space: 4 } },
              children: [new TextRun({ text: "Bao cao ky thuat - He thong phan tich log va phat hien bat thuong", font: FONT, size: 18, italics: true, color: "888888" })],
            }),
          ],
        }),
      },
      footers: {
        default: new Footer({
          children: [
            new Paragraph({
              alignment: AlignmentType.CENTER,
              border: { top: { style: BorderStyle.SINGLE, size: 4, color: "CCCCCC", space: 4 } },
              children: [
                new TextRun({ text: "Trang ", font: FONT, size: 18, color: "888888" }),
                new TextRun({ children: [PageNumber.CURRENT], font: FONT, size: 18, color: "888888" }),
              ],
            }),
          ],
        }),
      },
      children: [

        // ── MUC LUC ──
        new Paragraph({
          alignment: AlignmentType.CENTER,
          spacing: { after: 400 },
          children: [new TextRun({ text: "MUC LUC", font: FONT, size: 32, bold: true, color: BLUE })],
        }),
        ...[
          ["1.", "Gioi thieu"],
          ["2.", "Kien truc tong the he thong"],
          ["3.", "Cong nghe su dung"],
          ["4.", "Pipeline xu ly du lieu"],
          ["5.", "Cac thuat toan phat hien bat thuong"],
          ["6.", "Toi uu hieu nang va kha nang chiu tai"],
          ["7.", "He thong giam sat va quan sat"],
          ["8.", "Kiem thu va dam bao chat luong"],
          ["9.", "Ket luan"],
        ].map(([num, title]) =>
          new Paragraph({
            spacing: { after: 120, line: 360 },
            tabStops: [{ type: TabStopType.RIGHT, position: TabStopPosition.MAX }],
            children: [
              new TextRun({ text: `${num}  ${title}`, font: FONT, size: 24 }),
            ],
          })
        ),

        new Paragraph({ children: [new PageBreak()] }),

        // ══════════════════════════════════════════════════════════════════
        // CHUONG 1: GIOI THIEU
        // ══════════════════════════════════════════════════════════════════
        heading1("1. Gioi thieu"),

        heading2("1.1. Boi canh va dong luc"),

        p("Trong boi canh cac he thong phan mem hien dai ngay cang phuc tap voi kien truc microservice, container hoa va dien toan dam may, luong du lieu log sinh ra tu cac dich vu tang truong theo cap so nhan. Mot he thong san xuat quy mo vua co the sinh ra hang trieu ban ghi log moi phut, bao gom thong tin ve cac yeu cau HTTP, ket qua xu ly giao dich, loi xac thuc, va menh lenh he thong. Viec phan tich thu cong luong du lieu nay de phat hien cac bat thuong nhu tan cong bao mat, suy giam hieu nang hoac loi dich vu la dieu bat kha thi ve mat thoi gian va nhan luc."),

        p("He thong phan tich log va phat hien bat thuong thoi gian thuc duoc trinh bay trong bao cao nay ra doi nham giai quyet chinh xac thach thuc do. He thong tiep nhan log co cau truc tu nhieu dich vu thong qua hang doi tin nhan Apache Kafka, xu ly chung qua mot pipeline da luong song song, ap dung sau quy tac phat hien bat thuong dua tren cua so truot thoi gian, va luu tru ket qua vao Elasticsearch de truy van va truc quan hoa. Toan bo quy trinh dien ra trong thoi gian thuc voi do tre tu luc nhan log den luc phat hien bat thuong chi tinh bang giay."),

        heading2("1.2. Muc tieu he thong"),

        p("He thong duoc thiet ke voi ba muc tieu chinh. Thu nhat, dam bao kha nang tiep nhan va xu ly hang tram nghin ban ghi log moi giay ma khong mat du lieu, thong qua co che at-least-once delivery va xu ly idempotent. Thu hai, phat hien tu dong sau loai bat thuong pho bien trong he thong phan mem phan tan, bao gom tang dot bien ty le loi, vuot nguong do tre phan hoi, loi lap lai cung mau, tan cong brute-force xac thuc, truy cap ngoai gio lam viec va dich vu ngung hoat dong. Thu ba, cung cap bo cong cu quan sat toan dien cho phep ky su van hanh theo doi trang thai he thong theo thoi gian thuc thong qua Kibana, Grafana va Prometheus."),

        new Paragraph({ children: [new PageBreak()] }),

        // ══════════════════════════════════════════════════════════════════
        // CHUONG 2: KIEN TRUC TONG THE
        // ══════════════════════════════════════════════════════════════════
        heading1("2. Kien truc tong the he thong"),

        heading2("2.1. Mo hinh kien truc huong su kien"),

        p("He thong duoc xay dung theo mo hinh kien truc huong su kien (event-driven architecture), trong do moi ban ghi log duoc coi la mot su kien doc lap di chuyen qua cac tang xu ly. Kien truc nay mang lai loi the quan trong ve kha nang mo rong va do tin cay: cac thanh phan san xuat log (producer) va thanh phan xu ly (consumer) duoc tach roi hoan toan thong qua hang doi tin nhan Kafka, cho phep moi ben hoat dong doc lap va co the duoc mo rong rieng le theo nhu cau."),

        p("Luong du lieu di chuyen mot chieu tu trai sang phai qua bon tang chinh. Tang thu nhat la tang tiep nhan (ingestion layer), noi Apache Kafka dong vai tro lam bo dem phan tan giua cac ung dung nguon va pipeline xu ly. Tang thu hai la tang xu ly (processing layer), noi ung dung Go cot loi thuc hien phan cu phap, lam giau du lieu va phat hien bat thuong. Tang thu ba la tang luu tru (storage layer), noi Elasticsearch luu tru ca log va anomaly voi co che phan index theo ngay va truy van toan van. Tang thu tu la tang truc quan hoa (visualization layer), bao gom Kibana de duyet log, Grafana de hien thi dashboard metric va Prometheus de thu thap chi so hieu nang."),

        heading2("2.2. Cac thanh phan chinh"),

        p("He thong bao gom tam thanh phan duoc trieu khai duoi dang container Docker, duoc dieu phoi boi Docker Compose. Bang duoi day tom tat vai tro va cau hinh cua tung thanh phan."),

        spacer(),
        table(
          ["Thanh phan", "Cong nghe", "Port", "Vai tro"],
          [
            ["Message Queue", "Apache Kafka 7.5 (KRaft)", "9092", "Hang doi log dau vao, 3 partitions"],
            ["Storage", "Elasticsearch 9.0.0", "9200", "Luu tru va truy van log + anomaly"],
            ["Core App", "Go 1.25 (chi + franz-go)", "8080, 2112", "Pipeline xu ly va phat hien"],
            ["Log Viewer", "Kibana 9.0.0", "5601", "Giao dien duyet log (ELK)"],
            ["Dashboard", "Grafana 12.0.1", "3000", "Dashboard metric va log explorer"],
            ["Metrics", "Prometheus 3.4.0", "9090", "Thu thap va luu tru metric"],
            ["Init: Kafka", "confluent-local", "-", "Tao topic tu dong"],
            ["Init: Kibana", "curlimages/curl", "-", "Tao data view tu dong"],
          ],
          [1800, 2200, 1200, 4160]
        ),
        spacer(),

        p("Toan bo he thong duoc thiet ke de khoi dong tu dong thong qua mot lenh duy nhat. Cac init container dam bao rang topic Kafka va data view Kibana duoc tao san khi he thong bat dau lan dau. Health check duoc cau hinh cho moi dich vu de dam bao thu tu khoi dong dung: Elasticsearch va Kafka phai san sang truoc khi ung dung chinh bat dau, Kibana phai san sang truoc khi kibana-init chay, va Prometheus chi bat dau sau khi ung dung chinh da bao cao trang thai healthy."),

        new Paragraph({ children: [new PageBreak()] }),

        // ══════════════════════════════════════════════════════════════════
        // CHUONG 3: CONG NGHE SU DUNG
        // ══════════════════════════════════════════════════════════════════
        heading1("3. Cong nghe su dung"),

        heading2("3.1. Apache Kafka va mo hinh KRaft"),

        p("Apache Kafka duoc lua chon lam tang tiep nhan log nho kha nang xu ly thong luong cao, do ben du lieu va co che phan tan san co. Trong he thong nay, Kafka duoc trieu khai o che do KRaft (Kafka Raft), mot kien truc moi loai bo su phu thuoc vao Apache ZooKeeper bang cach su dung giao thuc dong thuan Raft duoc tich hop truc tiep vao broker. Dieu nay don gian hoa dang ke viec trieu khai va van hanh, dong thoi giam so luong thanh phan can quan ly."),

        p("Topic **application-logs** duoc cau hinh voi ba partition va he so sao chep bang mot (phu hop voi moi truong phat trien don node). Viec su dung ba partition cho phep ba consumer doc song song, tang thong luong xu ly len gap ba lan so voi mot partition duy nhat. Kafka broker duoc toi uu voi bon luong I/O, ba luong mang, va bo dem socket 1MB cho ca gui va nhan, dam bao hieu nang truyen tai cao ngay ca khi tai nang."),

        p("Ve mat ngu nghia giao nhan, he thong ap dung co che **at-least-once delivery**: offset cua moi message chi duoc danh dau (mark commit) sau khi message da duoc gui thanh cong vao Go channel noi bo. Dieu nay dam bao rang ngay ca khi consumer gap su co va khoi dong lai, khong co message nao bi mat. Cac message co the duoc xu ly trung lap, nhung co che document ID idempotent cua Elasticsearch (mo ta chi tiet o Muc 4) dam bao rang viec xu ly lai khong tao ra ban ghi trung lap."),

        heading2("3.2. Elasticsearch va chien luoc danh muc"),

        p("Elasticsearch 9.0.0 duoc su dung lam tang luu tru va truy van chinh, phu hop voi yeu cau tim kiem toan van (full-text search) va loc da dieu kien tren du lieu log co cau truc. He thong su dung hai loai index voi chien luoc phan vung khac nhau. Index log duoc phan theo ngay voi dinh dang **logs-YYYY.MM.DD** (vi du: logs-2026.04.12), cho phep quan ly vong doi du lieu de dang bang cach xoa cac index cu khi khong con can thiet. Index anomaly su dung mot index co dinh duy nhat ten la **anomalies** do luong anomaly thap hon nhieu so voi log."),

        p("Ca hai index deu duoc cau hinh voi **dynamic mapping disabled** (dynamic: false), nghia la chi nhung truong duoc dinh nghia truoc trong index template moi duoc danh muc. Chien luoc nay mang lai hai loi ich quan trong. Thu nhat, ngan chan viec Elasticsearch tu dong tao mapping cho cac truong bat ngo, tranh tinh trang mapping explosion khi du lieu co cau truc khong dong nhat. Thu hai, giam kich thuoc index va tang toc do ghi nho viec khong danh muc cac truong khong can thiet."),

        p("Cac truong trong log duoc danh muc voi kieu du lieu toi uu: **@timestamp** (date) cho phep truy van theo khoang thoi gian, **level** va **service** (keyword) cho phep loc chinh xac, **message** (text) ho tro tim kiem toan van, va **fields** (flattened) cho phep truy van tren cac truong dong ma khong tao mapping rieng cho tung truong con. Refresh interval duoc tang tu gia tri mac dinh 1 giay len 5 giay, giam so luong segment duoc tao ra gap 5 lan va cai thien dang ke thong luong ghi."),

        heading2("3.3. Ngon ngu Go va mo hinh dong thoi"),

        p("Go (Golang) duoc lua chon lam ngon ngu phat trien ung dung cot loi nho mo hinh dong thoi lightweight dua tren goroutine va channel. Khac voi cac mo hinh luong truyen thong (thread) cua Java hay C++, goroutine cua Go co chi phi tao va chuyen doi ngu canh cuc thap (chi vai kilobyte stack ban dau), cho phep he thong chay hang tram goroutine dong thoi ma khong gay ap luc dang ke len bo nho hay CPU. Channel cung cap co che truyen tin nhan an toan giua cac goroutine ma khong can su dung khoa tuong ho (mutex) trong hau het cac truong hop, don gian hoa dang ke viec lap trinh dong thoi."),

        p("He thong su dung thu vien **franz-go** de tuong tac voi Kafka, mot client Kafka thuan Go cung cap hieu nang cao va ho tro day du giao thuc Kafka. Thu vien **chi** duoc su dung lam HTTP router cho REST API nho giao dien gon gang va bo middleware phong phu (recovery, request ID, timeout, structured logging). Thu vien **go-elasticsearch** (phien ban 9) duoc su dung de tuong tac voi Elasticsearch, tan dung **BulkIndexer** co san de ghi du lieu theo lo (batch) mot cach hieu qua. Tat ca cac goroutine duoc dieu phoi boi **errgroup** tu goi golang.org/x/sync, dam bao rang loi o bat ky goroutine nao cung duoc phoi hop xu ly va tat ca goroutine duoc shutdown gracefully."),

        heading2("3.4. He sinh thai quan sat"),

        p("Bo ba cong cu quan sat bao gom Kibana, Grafana va Prometheus duoc tich hop de cung cap tam nhin toan dien ve trang thai he thong. **Kibana 9.0.0** dong vai tro lam giao dien chinh de duyet va tim kiem log, voi hai data view duoc tao tu dong: Application Logs (logs-*) va Anomalies. Nguoi dung co the su dung cu phap KQL (Kibana Query Language) de loc log theo dich vu, muc do nghiem trong, dia chi IP nguon va nhieu tieu chi khac. **Grafana 12.0.1** cung cap cac dashboard truc quan hoa voi du lieu tu ca Prometheus (metric he thong) va Elasticsearch (log va anomaly). **Prometheus 3.4.0** thu thap metric tu ung dung moi 15 giay, bao gom ty le tiep nhan log, do tre xu ly, so luong anomaly phat hien, do tre consumer Kafka va loi ghi Elasticsearch."),

        new Paragraph({ children: [new PageBreak()] }),

        // ══════════════════════════════════════════════════════════════════
        // CHUONG 4: PIPELINE XU LY DU LIEU
        // ══════════════════════════════════════════════════════════════════
        heading1("4. Pipeline xu ly du lieu"),

        heading2("4.1. Kafka Consumer va co che doc tin nhan"),

        p("Tang dau tien cua pipeline la Kafka Consumer, duoc hien thuc bang thu vien franz-go. Consumer hoat dong trong mot goroutine chuyen dung, goi ham PollFetches theo vong lap vo han de lay cac ban ghi tu Kafka broker. Cau hinh fetch duoc toi uu voi **FetchMaxBytes = 10MB** (gioi han tong kich thuoc moi lan fetch), **FetchMaxPartitionBytes = 1MB** (gioi han tren moi partition) va **MaxConcurrentFetches = 3** (cho phep ba yeu cau fetch dong thoi de bao hoa bang thong I/O). Cac ban ghi sau khi doc duoc chuyen thanh cau truc RawMessage gom payload (noi dung byte), topic, partition, offset va timestamp, roi duoc day vao mot Go channel co buffer 10.000 phan tu."),

        p("Co che offset commit duoc thiet ke theo nguyen tac an toan nhat: offset chi duoc danh dau (MarkCommitRecords) sau khi message da duoc gui thanh cong vao channel. Neu channel day (10.000 phan tu chua xu ly), consumer se bi block cho den khi co cho trong, tao ap luc nguoc (backpressure) tu nhien len nguon du lieu thay vi mat message. Khi consumer group bi rebalance (vi du khi them hay bot consumer instance), callback OnPartitionsRevoked duoc goi de commit tat ca cac offset da danh dau, dam bao khong mat tien trinh xu ly."),

        heading2("4.2. Pipeline Worker va xu ly song song"),

        p("Bon goroutine pipeline worker doc dong thoi tu cung mot channel. Go runtime tu dong phan phoi message den worker nao dang san sang, tao nen mo hinh fan-out tu nhien ma khong can bo phan phoi tuong minh. Moi worker thuc hien ba buoc xu ly cho moi message. Buoc thu nhat la **phan cu phap** (parsing): payload JSON duoc giai ma thanh cau truc LogEntry bao gom timestamp, level, service, message va cac truong mo rong. Neu JSON khong hop le, he thong khong loai bo message ma thay vao do tao mot LogEntry dang plain-text voi noi dung nguyen ban, dam bao rang khong ban ghi nao bi mat. Muc do log duoc chuan hoa (normalize) sang nam gia tri tieu chuan: error, warn, info, debug va unknown."),

        p("Buoc thu hai la **danh muc log** vao Elasticsearch. LogIndexer su dung BulkIndexer de gom cac yeu cau ghi thanh lo (batch), gui di khi dat nguong 10MB hoac sau moi 5 giay. Document ID duoc tinh bang ham bam **SHA-256** cua chuoi \"partition:offset\", tao ra mot dinh danh tat dinh (deterministic) va duy nhat cho moi message. Tinh chat nay dam bao **idempotent processing**: neu cung mot message bi xu ly lai (do consumer restart), no se ghi de len chinh document cu trong Elasticsearch thay vi tao ban ghi trung lap. Ten index duoc xac dinh tu timestamp cua log entry theo dinh dang logs-YYYY.MM.DD, tu dong phan vung du lieu theo ngay."),

        p("Buoc thu ba la **phat hien bat thuong**: log entry duoc chuyen cho Detection Engine de danh gia qua tat ca sau quy tac phat hien. Chi tiet ve cac thuat toan phat hien duoc trinh bay o Chuong 5."),

        heading2("4.3. Anomaly Dispatcher va mo hinh fan-out"),

        p("Khi Detection Engine phat hien mot bat thuong, anomaly duoc gui vao mot channel buffer 1.024 phan tu. Anomaly Dispatcher chay trong mot goroutine rieng, doc tu channel nay va thuc hien hai hanh dong song song cho moi anomaly: luu tru vao Elasticsearch (index anomalies) va gui canh bao qua AlertChannel. Hien tai AlertChannel chua duoc ket noi (nil), nhung giao dien da duoc thiet ke san de tich hop voi Slack, email, PagerDuty hoac bat ky kenh canh bao nao khac. Dispatcher duoc thiet ke de chiu loi: neu mot trong hai hanh dong that bai, loi chi duoc ghi log va quy trinh tiep tuc xu ly anomaly tiep theo, khong lam gian doan toan bo pipeline."),

        new Paragraph({ children: [new PageBreak()] }),

        // ══════════════════════════════════════════════════════════════════
        // CHUONG 5: THUAT TOAN PHAT HIEN BAT THUONG
        // ══════════════════════════════════════════════════════════════════
        heading1("5. Cac thuat toan phat hien bat thuong"),

        heading2("5.1. Cau truc du lieu cua so truot (Sliding Window)"),

        p("Nen tang cua bon trong sau quy tac phat hien la cau truc du lieu **cua so truot** (sliding window), duoc hien thuc bang ky thuat **vong dem tron** (circular buffer). Moi cua so truot bao gom mot mang timestamp co dung luong co dinh (10.000 phan tu), mot con tro head chi vi tri ghi tiep theo, va mot bien dem count theo doi so luong phan tu hop le. Khi phan tu moi duoc them vao, no ghi de len vi tri head theo phep toan modular (head mod capacity), sau do tang head va count. Khi buffer day, cac phan tu cu nhat tu dong bi ghi de, tao nen mot cua so truot tu nhien ma khong can thao tac xoa tuong minh."),

        p("Phep dem CountWithin(duration) duyet toan bo cac phan tu hop le va dem nhung phan tu co timestamp nam trong khoang thoi gian chi dinh. Thay vi dung tai vi tri dau tien nam ngoai cua so (chien luoc toi uu cho du lieu co thu tu), thuat toan quet toan bo buffer. Quyet dinh thiet ke nay duoc dua ra co chu dich de xu ly tinh huong **message khong theo thu tu** (out-of-order delivery) tu Kafka, dac biet khi consumer doc tu nhieu partition dong thoi. Do phuc tap thoi gian la O(n) voi n la so phan tu trong buffer, va do phuc tap khong gian la O(1) phu tro."),

        p("Co che don dep (eviction) duoc goi dinh ky moi 30 giay boi Detection Engine. Ham Evict() loc bo cac phan tu co timestamp cu hon cua so thoi gian, nen bo cac phan tu con lai ve dau mang va cap nhat lai head va count. Thao tac nay co do phuc tap O(n) nhung chi xay ra dinh ky, khong anh huong den hieu nang xu ly tung message."),

        heading2("5.2. Quy tac 1: Tang dot bien ty le loi (Error Rate Spike)"),

        p("Quy tac nay phat hien khi mot dich vu cu the phat sinh so luong log loi vuot qua nguong cho phep trong mot khoang thoi gian. He thong duy tri mot bang bam (hash map) anh xa tu ten dich vu sang cua so truot tuong ung. Khi mot log entry co level \"error\" den, he thong truy xuat hoac tao moi cua so truot cho dich vu do, them timestamp vao cua so, roi dem so loi trong khoang 5 phut gan nhat. Neu so dem dat hoac vuot nguong 10 loi, mot anomaly duoc phat sinh voi muc do nghiem trong HIGH. Gia tri nguong va do dai cua so deu co the cau hinh."),

        heading2("5.3. Quy tac 2: Vuot nguong do tre (Latency Threshold Breach)"),

        p("Quy tac nay phat hien khi ty le yeu cau co do tre phan hoi vuot qua nguong cho phep dat muc bao dong. Khac voi cua so truot thong thuong chi luu timestamp, quy tac nay su dung mot **cua so truot hai chieu** (latency window) voi moi phan tu gom ca timestamp va co breached (true/false chi ra do tre co vuot nguong hay khong). Khi mot log entry den, he thong trich xuat gia tri do tre tu truong **latency_ms** hoac **duration_ms** trong metadata, ho tro nhieu kieu du lieu (float64, int, string). Neu do tre bang hoac vuot 500ms, phan tu duoc danh dau la breached."),

        p("Ham StatsWithin() quet toan bo cua so va tra ve hai gia tri: tong so yeu cau va so yeu cau bi breach trong khoang thoi gian 5 phut. Ty le breach duoc tinh bang cong thuc: **breach_rate = (so_breach x 100) / tong_so_yeu_cau**. Neu ty le nay dat hoac vuot 20%, anomaly duoc phat sinh voi muc do MEDIUM. Thuat toan nay phat hien duoc tinh huong suy giam hieu nang co he thong (vi du khi co so du lieu cham lai) ma khong bi kich hoat boi mot vai yeu cau don le co do tre cao."),

        heading2("5.4. Quy tac 3: Loi lap lai cung mau (Repeated Failure)"),

        p("Quy tac nay phat hien khi cung mot loai loi xuat hien nhieu lan trong thoi gian ngan, goi y mot van de co he thong chua duoc xu ly. Thach thuc chinh la xac dinh \"cung mot loai loi\" khi noi dung thong bao loi thuong chua cac gia tri dong nhu ID giao dich, ma hex hay so thu tu. De giai quyet, he thong su dung ky thuat **van tay thong bao** (message fingerprinting) voi bieu thuc chinh quy: tat ca cac so thap phan va chuoi hex tu 4 ky tu tro len trong thong bao loi duoc thay the bang ky tu \"X\". Vi du, thong bao \"connection timeout for request abc1234 after 5000ms\" duoc chuyen thanh \"connection timeout for request X after Xms\". Hai thong bao co cung van tay sau chuan hoa duoc coi la cung loai loi."),

        p("He thong su dung bang bam voi khoa la to hop \"service|fingerprint\" va gia tri la cua so truot rieng cho tung cap. Khi mot log loi den, van tay duoc tinh toan, sau do so dem trong cua so 5 phut duoc kiem tra. Neu so lan lap lai dat hoac vuot 5, anomaly duoc phat sinh voi muc do MEDIUM. Co che don dep dinh ky xoa cac entry co so dem bang 0 sau khi eviction, ngan chan su tang truong khong gioi han cua bang bam khi cac loi cu khong con xuat hien."),

        heading2("5.5. Quy tac 4: Tan cong xac thuc (Auth Failure Burst)"),

        p("Quy tac nay phat hien cac cuoc tan cong brute-force xac thuc bang cach theo doi so lan dang nhap that bai tu cung mot nguon. He thong nhan dien loi xac thuc bang hai phuong phap: kiem tra thong bao loi co chua cac tu khoa \"auth\", \"login\" hoac \"unauthorized\" (khong phan biet hoa thuong), hoac kiem tra truong **auth_result** trong metadata co gia tri \"failure\". Phuong phap kep nay dam bao phat hien ca cac ung dung ghi log theo cac quy uoc khac nhau."),

        p("Khoa theo doi duoc lua chon theo thu tu uu tien: dia chi IP nguon (tu truong source_ip) duoc uu tien vi mot ke tan cong thuong su dung cung mot IP cho nhieu lan thu; neu khong co IP, he thong chuyen sang theo doi theo ten nguoi dung (tu truong username). Cua so truot 5 phut voi nguong 10 lan that bai duoc ap dung cho moi khoa. Khi nguong bi vuot, anomaly duoc phat sinh voi muc do HIGH, cho phep doi ung pho bao mat hanh dong kip thoi."),

        heading2("5.6. Quy tac 5: Truy cap ngoai gio (Off-Hours Access)"),

        p("Day la quy tac **khong trang thai** (stateless) duy nhat trong he thong, khong su dung cua so truot ma danh gia tung log entry doc lap. Quy tac kiem tra dong thoi hai dieu kien: duong dan truy cap thuoc danh sach nhay cam (mac dinh bao gom /admin, /api/v1/users va /internal), va thoi diem truy cap nam ngoai khung gio lam viec (mac dinh tu 9 gio sang den 5 gio chieu UTC). Ca danh sach duong dan nhay cam, khung gio lam viec va mui gio deu co the cau hinh. Khi ca hai dieu kien duoc thoa man, anomaly duoc phat sinh ngay lap tuc voi muc do MEDIUM. Do phuc tap thoi gian la O(p) voi p la so duong dan nhay cam (phep so sanh tien to chuoi), va khong gian phu la O(1) vi khong luu tru trang thai."),

        heading2("5.7. Quy tac 6: Dich vu ngung hoat dong (Service Silence)"),

        p("Quy tac nay phat hien khi mot dich vu truoc do dang hoat dong bat ngo ngung gui log, goi y rang dich vu co the da gap su co nghiem trong. Day la quy tac phuc tap nhat trong he thong, su dung mo hinh **hai pha** khac biet voi cac quy tac khac. Pha thu nhat (Evaluate) duoc goi boi pipeline worker moi khi co log entry den: he thong tang bo dem log cho dich vu tuong ung, va chi bat dau theo doi thoi gian gui log cuoi cung khi so dem dat nguong **MinLogCount** (mac dinh la 5). Co che \"cold-start guard\" nay ngan chan canh bao gia khi mot dich vu moi bat dau va chua kip gui du log."),

        p("Pha thu hai (CheckSilence) duoc goi boi mot goroutine heartbeat rieng moi 30 giay. Goroutine nay duyet toan bo danh sach dich vu dang theo doi, va neu thoi gian tu log cuoi cung den hien tai vuot qua **SilenceAfter** (mac dinh 5 phut), mot anomaly duoc phat sinh voi muc do CRITICAL -- muc cao nhat trong he thong. Dich vu sau do bi xoa khoi danh sach theo doi va phai tich luy lai MinLogCount log truoc khi bi theo doi tro lai. Toan bo trang thai duoc bao ve boi mot **sync.Mutex** vi ca Evaluate (goi tu nhieu pipeline worker) va CheckSilence (goi tu heartbeat goroutine) deu truy cap vao cung mot cau truc du lieu."),

        heading2("5.8. Co che warmup, cooldown va eviction"),

        p("Detection Engine ap dung ba co che bao ve de giam thieu canh bao gia va canh bao trung lap. **Warmup** la khoang thoi gian im lang sau khi he thong khoi dong, duoc tinh bang cong thuc: warmup_duration = warmup_multiplier x max(tat ca cua so thoi gian cua cac quy tac). Voi cau hinh mac dinh (multiplier = 2, cua so lon nhat = 5 phut), warmup keo dai 10 phut. Trong thoi gian nay, tat ca anomaly bi chan lai de cho phep cac cua so truot tich luy du du lieu dai dien, tranh tinh trang tang dot bien gia do du lieu lich su duoc nap lai tu offset cu nhat."),

        p("**Cooldown** hoat dong theo cap (rule_id, service): sau khi mot anomaly duoc phat sinh cho mot quy tac cu the tren mot dich vu cu the, cac anomaly tiep theo tu cung cap do bi chan trong 15 phut. Co che nay ngan chan tinh trang \"bao dong bao\" (alert storm) khi mot van de keo dai lien tuc sinh ra hang tram anomaly giong nhau. Trang thai cooldown duoc bao ve boi mutex rieng vi no duoc truy cap tu nhieu goroutine (pipeline workers va silence watcher)."),

        p("**Eviction loop** chay dinh ky moi 30 giay trong mot goroutine nen, thuc hien hai nhiem vu: goi Reset() tren tat ca cac detector de don dep du lieu cu khoi cac cua so truot, va xoa cac entry cooldown da het han (cu hon 2 lan cooldown duration). Co che nay dam bao rang bo nho su dung boi cac cua so truot va bang bam cooldown luon duoc kiem soat, khong tang truong vo han theo thoi gian."),

        new Paragraph({ children: [new PageBreak()] }),

        // ══════════════════════════════════════════════════════════════════
        // CHUONG 6: TOI UU HIEU NANG
        // ══════════════════════════════════════════════════════════════════
        heading1("6. Toi uu hieu nang va kha nang chiu tai"),

        heading2("6.1. Xu ly song song da cap"),

        p("He thong ap dung chien luoc xu ly song song o nhieu cap do de toi da hoa thong luong. O cap Kafka, ba partition cho phep ba consumer thread doc dong thoi (trong truong hop mo rong thanh consumer group nhieu instance). O cap pipeline, bon goroutine worker chia se cong viec tu mot channel chung, tan dung co che scheduling cua Go runtime de phan phoi tai tu dong. O cap Elasticsearch, BulkIndexer su dung bon worker thread noi bo de gui cac yeu cau bulk song song. Su dung nhieu cap song song nhu vay cho phep he thong tan dung toi da tai nguyen CPU da nhan va bang thong mang."),

        p("Channel co buffer dong vai tro quan trong trong viec can bang toc do giua cac tang. Consumer channel voi buffer 10.000 message cho phep Kafka consumer tiep tuc doc ngay ca khi pipeline worker tam thoi cham lai, hoat dong nhu mot bo giam xoc (shock absorber). Tuong tu, anomaly channel voi buffer 1.024 phan tu cho phep Detection Engine tiep tuc xu ly ma khong bi block boi Dispatcher. Khi channel day, he thong ap dung chien luoc tu dong: consumer bi block (backpressure), con anomaly bi drop voi canh bao log (chap nhan mat anomaly hon la lam cham pipeline)."),

        heading2("6.2. Ghi du lieu theo lo va idempotent processing"),

        p("BulkIndexer cua Elasticsearch la mot trong nhung toi uu quan trong nhat cua he thong. Thay vi gui tung yeu cau ghi rieng le (co the tao ra hang nghin yeu cau HTTP moi giay), BulkIndexer gom cac yeu cau lai thanh cac lo (batch) va gui mot lan. Lo duoc gui khi dat mot trong hai dieu kien: tong kich thuoc dat 10MB hoac da troi qua 5 giay ke tu lan gui truoc. Chien luoc nay giam so luong round-trip HTTP xuong hang tram lan, dong thoi cho phep Elasticsearch toi uu hoa viec ghi noi bo (vi du: gop cac segment Lucene)."),

        p("Document ID duoc tinh bang ham bam **SHA-256** cua chuoi \"partition:offset\" dam bao tinh **idempotent**: cung mot message Kafka luon tao ra cung mot document ID trong Elasticsearch. Neu message bi xu ly lai (do consumer restart, rebalance hay loi mang), no se ghi de len chinh document cu thay vi tao ban ghi moi. Tinh chat nay cuc ky quan trong trong he thong phan tan noi loi co the xay ra o bat ky thoi diem nao, va ngu nghia at-least-once delivery cua Kafka dong nghia voi viec message co the duoc gui nhieu lan."),

        heading2("6.3. Cau truc du lieu toi uu bo nho"),

        p("Cac cua so truot su dung ky thuat vong dem tron (circular buffer) voi dung luong co dinh 10.000 phan tu, dam bao rang bo nho su dung boi moi cua so la hang so O(1) bat ke he thong chay bao lau. Khi buffer day, phan tu cu nhat tu dong bi ghi de boi phan tu moi ma khong can thao tac cap phat hay giai phong bo nho. Dieu nay loai bo hoan toan ap luc len bo thu gom rac (garbage collector) cua Go, mot yeu to quan trong khi xu ly hang tram nghin message moi giay."),

        p("Bang bam su dung khoa dang string (ten dich vu, van tay loi, dia chi IP) cung cap truy cap O(1) trung binh. Co che don dep dinh ky xoa cac entry co so dem bang 0, ngan chan tinh trang bang bam tang truong vo han khi cac nguon loi cu khong con xuat hien. ServiceSilenceRule su dung nguong stale gap ba lan silence duration de quyet dinh khi nao xoa hoan toan mot dich vu khoi danh sach theo doi, can bang giua hieu qua bo nho va do chinh xac phat hien."),

        heading2("6.4. Index template va cau hinh Elasticsearch"),

        p("Index template voi **zero replicas** la phu hop cho moi truong don node, loai bo overhead sao chep du lieu. **Refresh interval 5 giay** (thay vi mac dinh 1 giay) giam so luong segment Lucene duoc tao ra gap 5 lan, truc tiep cai thien thong luong ghi. Tuy nhien, dieu nay dong nghia voi viec du lieu moi se khong xuat hien trong ket qua truy van cho den 5 giay sau khi ghi, mot danh doi chap nhan duoc cho he thong phan tich log (khong yeu cau do tre truy van duoi giay). **Thread pool write queue** duoc tang len 1.000 va **search queue** duoc tang len 500, cho phep Elasticsearch xu ly nhieu yeu cau dong thoi hon truoc khi bat dau tu choi."),

        new Paragraph({ children: [new PageBreak()] }),

        // ══════════════════════════════════════════════════════════════════
        // CHUONG 7: GIAM SAT VA QUAN SAT
        // ══════════════════════════════════════════════════════════════════
        heading1("7. He thong giam sat va quan sat"),

        heading2("7.1. Prometheus metric va canh bao"),

        p("Ung dung Go cot loi xuat bay chi so hieu nang (metric) qua endpoint Prometheus tai cong 2112. Sau metric chinh duoc theo doi. **logs_consumed_total** (counter voi nhan topic va partition) dem tong so log da doc tu Kafka, cho phep tinh toan toc do tiep nhan theo thoi gian. **logs_processed_duration_seconds** (histogram) do do tre xu ly end-to-end cua moi message, tu do co the tinh cac phan vi p50, p95 va p99. **anomalies_detected_total** (counter voi nhan rule) dem so anomaly da phat hien, phan loai theo quy tac. **kafka_consumer_lag** (gauge voi nhan topic va partition) do khoang cach giua offset moi nhat va offset da xu ly, chi ra pipeline co dang theo kip toc do du lieu den hay khong. **elasticsearch_write_errors_total** (counter) dem so loi ghi Elasticsearch. **parse_errors_total** (counter) dem so log khong the phan cu phap."),

        heading2("7.2. Truc quan hoa voi Grafana va Kibana"),

        p("Grafana cung cap hai dashboard duoc tao tu dong khi he thong khoi dong. Dashboard **Log Analytics & Anomaly Detection** hien thi bay panel bao gom: ty le tiep nhan log theo thoi gian, do tre xu ly (p50/p95/p99), ty le phat hien anomaly theo quy tac, bieu do phan bo anomaly, do tre consumer Kafka, so loi ghi Elasticsearch va so loi phan cu phap. Dashboard **Log Explorer** su dung Elasticsearch lam nguon du lieu truc tiep, hien thi luong log theo thoi gian, phan bo theo dich vu va muc do, cung voi bang chi tiet anomaly."),

        p("Kibana dong vai tro lam giao dien chinh de duyet va tim kiem log chi tiet. Thong qua trang Discover, nguoi dung co the xem tung dong log voi tat ca cac truong metadata, su dung cu phap KQL de loc (vi du: service : \"payment-service\" and level : \"error\"), va phan tich xu huong bang cac bieu do tich hop san. Hai data view Application Logs va Anomalies duoc tao tu dong boi kibana-init container, cho phep nguoi dung bat dau su dung ngay ma khong can cau hinh thu cong."),

        new Paragraph({ children: [new PageBreak()] }),

        // ══════════════════════════════════════════════════════════════════
        // CHUONG 8: KIEM THU
        // ══════════════════════════════════════════════════════════════════
        heading1("8. Kiem thu va dam bao chat luong"),

        heading2("8.1. Cong cu tao tai (Load Test)"),

        p("He thong cung cap cong cu tao tai (cmd/loadtest) duoc thiet ke de mo phong chinh xac cac kich ban bat thuong trong thuc te. Cong cu chay trong 5 phut voi bay pha lien tiep, moi pha sinh ra mot loai du lieu cu the de kich hoat mot quy tac phat hien tuong ung. Pha warm-up tao traffic binh thuong de cua so truot tich luy du lieu nen. Cac pha tiep theo lan luot tao ra tang dot bien loi tren payment-service, tan cong xac thuc tu IP 10.66.6.6, yeu cau co do tre cao tren api-gateway, loi lap lai tren order-service, truy cap duong dan nhay cam ngoai gio, va cuoi cung la pha wind-down noi inventory-service ngung gui log. Bon goroutine producer hoat dong song song voi toc do co the cau hinh (mac dinh 500 message/giay), su dung rate multiplier theo pha de mo phong cac muc tai khac nhau."),

        heading2("8.2. Kiem thu toan dien end-to-end (E2E Test)"),

        p("Cong cu kiem thu end-to-end (cmd/e2etest) mo rong cong cu tao tai bang viec tu dong xac nhan ket qua. Cong cu thuc hien nam buoc tuan tu. Buoc dau kiem tra suc khoe he thong qua endpoint /ready. Buoc hai ghi nhan so luong log va anomaly hien tai lam moc so sanh. Buoc ba day log vao Kafka theo sau pha, dong thoi lay mau do tre pipeline moi 10 giay bang cach truy van API de so sanh so luong log da gui voi so luong da danh muc. Buoc bon doi pipeline xu ly het bang cach poll API lien tuc cho den khi so luong log on dinh hoac dat 100%. Buoc nam tong hop ket qua thanh bao cao chi tiet bao gom thong luong producer, ty le danh muc, thoi gian drain, va bang phan tich anomaly theo tung quy tac, ket thuc voi phan dinh PASS (tu 99%), WARN (95-99%) hoac FAIL (duoi 95%)."),

        heading2("8.3. Kiem thu don vi va tich hop"),

        p("Tat ca cac thanh phan chinh deu co bo kiem thu don vi (unit test) kem theo, su dung framework testing tieu chuan cua Go ket hop voi thu vien testify cho cac assertion va mock. Cac test cho API handler su dung mock store de kiem tra logic xu ly yeu cau ma khong can Elasticsearch thuc. Cac test cho Elasticsearch indexer su dung mock HTTP transport de kiem tra logic ghi va doc. Cac test cho Detection Engine kiem tra tung quy tac rieng le voi cac kich ban bao gom truong hop binh thuong, truong hop gioi han (boundary case) va truong hop loi. Bo test tich hop su dung testcontainers-go de khoi dong Kafka va Elasticsearch thuc trong container Docker, kiem tra toan bo luong tu producer den consumer den indexer."),

        new Paragraph({ children: [new PageBreak()] }),

        // ══════════════════════════════════════════════════════════════════
        // CHUONG 9: KET LUAN
        // ══════════════════════════════════════════════════════════════════
        heading1("9. Ket luan"),

        p("Bao cao nay da trinh bay chi tiet ve thiet ke, hien thuc va cac phuong phap toi uu cua he thong phan tich log va phat hien bat thuong thoi gian thuc. He thong su dung kien truc huong su kien voi Apache Kafka lam tang tiep nhan, pipeline da luong trong Go lam tang xu ly, Elasticsearch lam tang luu tru, va bo ba Kibana-Grafana-Prometheus lam tang quan sat. Sau quy tac phat hien bat thuong duoc hien thuc dua tren cau truc du lieu cua so truot voi vong dem tron, bao phu cac tinh huong pho bien tu tang dot bien ty le loi, vuot nguong do tre, loi lap lai, tan cong xac thuc, truy cap ngoai gio, den dich vu ngung hoat dong."),

        p("Cac phuong phap toi uu bao gom xu ly song song da cap (Kafka partition, pipeline worker, bulk indexer worker), ghi du lieu theo lo (bulk indexing voi nguong 10MB/5s), cau truc du lieu toi uu bo nho (vong dem tron dung luong co dinh), xu ly idempotent (SHA-256 document ID), va co che bao ve thong minh (warmup, cooldown, eviction). Nhung phuong phap nay cho phep he thong xu ly hang tram nghin ban ghi log moi giay tren mot node duy nhat, dong thoi duy tri do tre phat hien bat thuong o muc giay."),

        p("Huong phat trien tiep theo co the bao gom tich hop AlertChannel voi cac kenh canh bao thuc te nhu Slack va PagerDuty, bo sung cac quy tac phat hien dua tren machine learning, ho tro mo rong ngang (horizontal scaling) voi nhieu consumer instance, va hien thuc co che retention tu dong cho cac index Elasticsearch cu. He thong da dat duoc muc do san sang san xuat (production-ready) voi day du cac thanh phan can thiet cho viec giam sat va phat hien bat thuong trong mot he thong phan tan quy mo vua."),

        spacer(),

        new Paragraph({
          alignment: AlignmentType.CENTER,
          spacing: { before: 600 },
          border: { top: { style: BorderStyle.SINGLE, size: 4, color: BLUE, space: 8 } },
          children: [new TextRun({ text: "--- Het ---", font: FONT, size: 22, italics: true, color: "888888" })],
        }),
      ],
    },
  ],
});

// ── Generate ─────────────────────────────────────────────────────────────────

Packer.toBuffer(doc).then((buffer) => {
  const outPath = process.argv[2] || "report.docx";
  fs.writeFileSync(outPath, buffer);
  console.log(`Generated: ${outPath} (${(buffer.length / 1024).toFixed(0)} KB)`);
});
