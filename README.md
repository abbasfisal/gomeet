# سرویس رزرو اتاق جلسات (Meeting Room Booking Service)

یک سرویس RESTful با زبان Go برای مدیریت اتاق‌های جلسات، تعریف زمان‌های در دسترس، و رزرو اتاق.

## تکنولوژی‌های استفاده شده

| مؤلفه | انتخاب |  
|--------|--------|
| زبان | Go 1.21+ |
| فریم‌ورک | Gin |
| ORM | GORM |
| دیتابیس | PostgreSQL | 
| کش | Redis |
| کانتینر | Docker + docker-compose |

## معماری پروژه

```
├── cmd/server/          # نقطه ورود برنامه
├── internal/
│   ├── config/          # تنظیمات از محیط (env)
│   ├── model/           # مدل‌های دیتابیس (GORM)
│   ├── repository/      # لایه دسترسی به داده
│   ├── service/         # لایه منطق کسب و کار
│   ├── handler/         # هندلرهای HTTP
│   ├── middleware/      # میدل‌ویر (لاگر)
│   ├── cache/           # کش Redis
│   └── router/          # تنظیم مسیرها
├── pkg/response/        # کمک‌کننده‌های پاسخ JSON
└── api-tests/           # مثال‌های curl
```

### لایه‌بندی (Clean Architecture)

1. **Repository**: ارتباط با دیتابیس از طریق GORM. تمام عملیات CRUD در این لایه انجام می‌شود.
2. **Cache**: کش کردن اطلاعات پرخوان (read-heavy) مثل لیست اتاق‌ها و زمان‌های در دسترس.
3. **Service**: منطق کسب و کار شامل:
   - بررسی تداخل رزروها (overlap detection)
   - محاسبه زمان‌های آزاد (free slot calculation)
   - اعتبارسنجی رزرو نسبت به زمان‌های در دسترس
   - پاک کردن کش پس از عملیات نوشتن (cache invalidation)
4. **Handler**: دریافت درخواست HTTP، اعتبارسنجی اولیه، ارسال به سرویس، برگرداندن پاسخ استاندارد.

## نحوه اجرا

```bash
docker compose up --build
```

این دستور سه سرویس را بالا می‌آورد:
- **app**: سرویس اصلی روی پورت 8080
- **db**: PostgreSQL روی پورت 5432
- **redis**: Redis روی پورت 6379

پس از اجرا، endpointها روی `http://localhost:8080` در دسترس هستند.

### متغیرهای محیطی (Environment Variables)

| متغیر | پیش‌فرض | توضیح |
|--------|---------|-------|
| SERVER_PORT | 8080 | پورت سرویس |
| DATABASE_URL | host=localhost ... | کانکشن دیتابیس |
| REDIS_URL | localhost:6379 | آدرس Redis |
| REDIS_PASSWORD | (خالی) | رمز Redis |
| REDIS_DB | 0 | شماره دیتابیس Redis |

## مدل داده (Data Model)

### Room
| فیلد | نوع | توضیح |
|------|-----|-------|
| id | uint (PK) | شناسه یکتا |
| name | string (unique) | نام اتاق |
| capacity | int | ظرفیت |
| location | string | مکان |
| description | string | توضیحات |
| created_at | timestamp | زمان ایجاد |
| updated_at | timestamp | زمان بروزرسانی |

### Availability
| فیلد | نوع | توضیح |
|------|-----|-------|
| id | uint (PK) | شناسه |
| room_id | uint (FK) | اتاق مرتبط |
| day_of_week | smallint (0-6) | روز هفته (اختیاری، برای تکرار هفتگی) |
| specific_date | date | تاریخ مشخص (اختیاری) |
| start_time | time | شروع زمان در دسترس |
| end_time | time | پایان زمان در دسترس |

**نکته:** حداقل یکی از `day_of_week` یا `specific_date` باید مقدار داشته باشد.

### Booking
| فیلد | نوع | توضیح |
|------|-----|-------|
| id | uint (PK) | شناسه |
| room_id | uint (FK) | اتاق |
| start_time | timestamp | شروع رزرو |
| end_time | timestamp | پایان رزرو |
| created_at | timestamp | زمان ایجاد |
| updated_at | timestamp | زمان بروزرسانی |

**Constraint (Exclusion):** برای جلوگیری از double-booking از `EXCLUDE USING gist (room_id WITH =, tstzrange(start_time, end_time) WITH &&)` استفاده شده که در سطح دیتابیس از تداخل رزروها جلوگیری می‌کند (نیازمند extension `btree_gist`).

### چرا این مدل را انتخاب کردم؟

- **Availability انعطاف‌پذیر**: پشتیبانی همزمان از زمان‌های تکراری هفتگی (day_of_week) و تاریخ‌های مشخص (specific_date). این امکان را می‌دهد که هم ساعات اداری تکراری و هم روزهای خاص (مثلاً تعطیلات) را مدیریت کنید.
- **Exclusion Constraint در PostgreSQL**: 

## جلوگیری از Double-Booking (Concurrency Safety)

سه لایه برای جلوگیری از double-booking پیاده‌سازی شده:

### 1. تراکنش دیتابیس (Transaction + Check)
در `repository/booking_repository.go`، متد `CreateWithOverlapCheck` داخل یک تراکنش دیتابیس ابتدا تعداد رزروهای همپوشان را چک می‌کند و در صورت عدم وجود، رزرو جدید را ایجاد می‌کند. از آنجایی که این عملیات در یک تراکنش انجام می‌شود، دو درخواست همزمان نمی‌توانند همزمان بنویسند.

### 2. Exclusion Constraint (PostgreSQL)
در سطح دیتابیس، یک `EXCLUDE CONSTRAINT` با استفاده از `btree_gist` روی فیلدهای `room_id` و `tsrange(start_time, end_time)` تعریف شده است. این constraint به صورت اتمی از درج رکوردهایی که بازه زمانی همپوشان دارند جلوگیری می‌کند.

```sql
ALTER TABLE bookings ADD CONSTRAINT no_overlap_bookings
EXCLUDE USING gist (room_id WITH =, tstzrange(start_time, end_time) WITH &&);
```

> نکته: چون GORM `time.Time` را به صورت `timestamp with time zone` (timestamptz) ذخیره می‌کند، باید از `tstzrange` استفاده شود؛ `tsrange` فقط `timestamp` (بدون timezone) می‌پذیرد و constraint در آن صورت ساخته نمی‌شود.

این دو لایه با هم (تراکنش + constraint) مشکل race condition را کاملاً حل می‌کنند. چک داخل تراکنش (لایه ۱) پاسخ سریع به overlapهای معمولی می‌دهد، اما در حالت race، **constraint دیتابیس مرجع نهایی است**؛ خطای `exclusion_violation` (SQLSTATE `23P01`) به `ErrOverlap` ترجمه می‌شود و در API به صورت **409 Conflict** برمی‌گردد.

### 3. اعتبارسنجی در سرویس
قبل از اقدام به رزرو، سرویس بررسی می‌کند که بازه زمانی درخواستی در محدوده availability اتاق قرار دارد (`isWithinAvailability`). پنجره‌های availability در یک روز ابتدا merge می‌شوند (بازه‌های هم‌پوشان یا پشت‌سرهم یک بازه پیوسته می‌شوند)، بنابراین رزرو می‌تواند از چند پنجره‌ی پشت‌سرهم رد شود ولی هیچ بخشی از آن نباید داخل gap بین پنجره‌ها بیفتد.

### اعتبارسنجی ورودی (Validation)

- **Availability**: هر رکورد باید حداقل یکی از `day_of_week` (0-6) یا `specific_date` را داشته باشد؛ `start_time`/`end_time` باید فرمت `HH:MM` معتبر باشند و `end_time` بعد از `start_time` باشد. مقادیر نامعتبر مثل `"25:99"` به جای نرمالایز شدن بی‌صدا، با خطای 400 رد می‌شوند.
- **Booking**: `end_time` باید بعد از `start_time` باشد و کل بازه باید داخل availability اتاق باشد.

## Redis Cache

### کلیدهای کش (Cache Keys)

| الگوی کلید | محتوا | TTL |
|-----------|-------|-----|
| `rooms:list:{page}:{size}` | لیست اتاق‌ها با صفحه‌بندی | ۵ دقیقه |
| `room:{id}` | جزئیات یک اتاق | ۵ دقیقه |
| `room:{id}:availability:{from}:{to}` | اسلات‌های آزاد یک اتاق | ۳ دقیقه |

### استراتژی Invalidation

- **ایجاد/به‌روزرسانی اتاق**: پاک کردن `rooms:*` و `room:{id}`
- **ایجاد/لغو رزرو**: پاک کردن فقط کلیدهای availability همان اتاق (`room:{room_id}:availability:*`)
- **حذف اتاق**: پاک کردن `rooms:*`, `room:{id}`, `room:{id}:availability:*`

از `Scan` با الگوی `*` برای پیدا کردن و حذف همه کلیدهای مرتبط استفاده می‌شود. برای رزرو، به جای پاک کردن کل کش availability همه‌ی اتاق‌ها، فقط الگوی `room:{room_id}:availability:*:*` همان اتاق پاک می‌شود (targeted invalidation).

## API Endpoints

| Method | Path | توضیح |
|--------|------|-------|
| POST | /api/rooms | ایجاد اتاق جدید (به همراه availability) |
| GET | /api/rooms | لیست اتاق‌ها (صفحه‌بندی شده) |
| GET | /api/rooms/:id | جزئیات یک اتاق |
| GET | /api/rooms/:id/availability | اسلات‌های آزاد (?from=&to= به صورت unix timestamp) |
| POST | /api/bookings | ایجاد رزرو جدید |
| GET | /api/bookings | لیست رزروها (?room_id=&from=&to=) |
| DELETE | /api/bookings/:id | لغو رزرو |

### ساختار پاسخ استاندارد

**موفقیت:**
```json
{ "data": { ... } }
```

**خطا:**
```json
{ "error": { "code": "NOT_FOUND", "message": "room not found" } }
```

### کدهای HTTP
- 200: موفقیت
- 201: ایجاد شده
- 400: درخواست نامعتبر
- 404: پیدا نشد
- 409: تداخل (double-booking) یا نام تکراری اتاق

## اجرای تست‌ها

```bash
go test ./... -v
```

تست‌های واحد (`internal/service`):
- `isWithinAvailability`: انواع حالت‌های مرزی، پنجره‌های پشت‌سرهم، gap بین پنجره‌ها
- `subtractBookings`: تفریق بازه‌های رزرو از بازه‌های در دسترس (شامل ورودی نامرتب)
- `calculateFreeSlots`: محاسبه اسلات‌های آزاد و عدم گزارش gap به عنوان اسلات آزاد
- `mergeIntervals`: ادغام پنجره‌های هم‌پوشان/پشت‌سرهم
- Parse، validation و helper functions

تست‌های ترجمه خطا (`internal/repository`): نگاشت SQLSTATE `23P01` به `ErrOverlap` و `23505` به `ErrConflict`.

**تست‌های Integration** (`internal/repository/booking_repository_integration_test.go`): با دیتابیس PostgreSQL واقعی اجرا می‌شوند و شامل تست همزمانی double-booking است (۲۰ درخواست همزمان برای یک بازه → فقط یکی موفق). اگر دیتابیس در دسترس نباشد به صورت خودکار skip می‌شوند:

```bash
docker compose up -d db   # سپس
go test ./internal/repository/ -run TestCreateWithOverlapCheck -v
```

DSN از متغیر `DATABASE_URL` (یا فایل `.env` در ریشه‌ی پروژه) خوانده می‌شود و پیش‌فرض آن با `docker-compose.yml` (پورت میزبان `5433`) هماهنگ است. برای اتصال به دیتابیس دیگری، کافی است:

```bash
DATABASE_URL="host=localhost user=meetroom password=meetroom dbname=meetroom port=5433 sslmode=disable" \
  go test ./internal/repository/ -run TestCreateWithOverlapCheck -v
```

### لاگ‌گذاری
- لاگ‌گذاری ساختاریافته با `log/slog` در قالب JSON: middleware ثبت درخواست‌ها (method, path, status, latency) و خطاهای داخلی بدون نشت جزئیات به کلاینت.
