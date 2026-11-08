# Daya Assistant

پنل فارسی مستقل برای تعریف، زمان‌بندی و مدیریت سفارش‌های خرید و فروش کارگزاری دایا. مدل سفارش و گردش‌کار عملیاتی از قابلیت‌های Rabin Assistant الگو گرفته، اما اتصال و احراز هویت کارگزاری به‌صورت مستقل برای دایا پیاده‌سازی شده است.

## امکانات

- سفارش خرید و فروش با قیمت، تعداد، نماد و ISIN
- ذخیره پایدار سفارش‌ها، زمان‌بندی و انتقال خودکار سفارش سررسیدشده به وضعیت «آماده بررسی»
- تقویم شمسی، ساعت تهران و زمان با دقت میلی‌ثانیه
- جست‌وجو و پیشنهاد نماد، فهرست سهام و صندوق‌ها و تطبیق دقیق نماد/ISIN
- برنامه‌ریز نوبت‌های سرخطی تا ۵۰۰۰ سفارش با فاصله زمانی قابل تنظیم
- تعیین تعداد سهم برای هر سفارش یا تقسیم تعداد کل بین نوبت‌ها
- فهرست سفارش‌های فعال و تاریخچه
- لغو تکی، لغو گروهی سفارش‌های زمان‌بندی‌شده، کپی سفارش و پاک‌کردن تاریخچه
- دو مسیر عملیاتی دستی و API (مسیر API تا زمان تأیید قرارداد سفارش دایا غیرفعال است)
- ورود اختصاصی دایا با دریافت CAPTCHA، ورود دستی پاسخ و ذخیره رمزنگاری‌شده نشست

## نصب روی Ubuntu

برای نصب Production با root:

```bash
bash -c "$(curl -fsSL https://raw.githubusercontent.com/hatinati2/daya-assistant/main/install-public.sh)"
```

برای نصب Preview مستقل:

```bash
bash -c "$(curl -fsSL https://raw.githubusercontent.com/hatinati2/daya-assistant/main/install-preview-public.sh)"
```

تا وقتی تغییرات شاخهٔ توسعه ادغام نشده‌اند، برای نصب همان نسخهٔ آزمایشی از دستور زیر استفاده کن:

```bash
DAYA_INSTALL_REF=feat/daya-order-dispatch-foundation bash -c "$(curl -fsSL https://raw.githubusercontent.com/hatinati2/daya-assistant/feat/daya-order-dispatch-foundation/install-preview-public.sh)"
```

این دستور فقط Preview مستقل را نصب می‌کند؛ پورت آن 8102 است. پس از ادغام PR می‌توان از نصب‌کنندهٔ `main` استفاده کرد.

| مورد | Production | Preview |
|---|---|---|
| پورت | 8100 | 8102 |
| سرویس | `daya-assistant.service` | `daya-assistant-preview.service` |
| پوشه برنامه | `/opt/daya-assistant` | `/opt/daya-assistant-preview` |
| داده‌ها | `/var/lib/daya-assistant` | `/var/lib/daya-assistant-preview` |
| تنظیمات | `/etc/daya-assistant.env` | `/etc/daya-assistant-preview.env` |
| کلید | `/etc/daya-assistant.key` | `/etc/daya-assistant-preview.key` |

پورت‌های Daya به‌صورت مستقل از رابین تنظیم شده‌اند: Production روی 8100 و Preview روی 8102. Preview مسیر داده، فایل تنظیمات، کلید رمزنگاری، کاربر سیستم و سرویس جداگانه دارد. نصب‌کننده Preview مقدار پیش‌فرض قدیمی 8101 را نیز به 8102 منتقل می‌کند، اما آدرس سفارشی موجود را تغییر نمی‌دهد.

اطلاعات ورود اولیه در فایل root-only ذخیره می‌شود: `/root/daya-assistant-credentials.txt` یا `/root/daya-assistant-preview-credentials.txt`. تنظیمات قبلی در به‌روزرسانی حفظ می‌شوند.

## احراز هویت دایا

قرارداد ورود از Network مرورگر استخراج شده است:
- `GET https://mg.dayatrader.ir/captcha`: تصویر PNG، هدر `captchaId` و کوکی `cookiesession1`.
- `POST https://mg.dayatrader.ir/oms-login`: فیلدهای `username`، `password`، `captchaAnswer` و `captchaId` همراه هدر `x-client-type: WEB_CLIENT` و همان کوکی CAPTCHA.
- پاسخ موفق، کوکی HttpOnly با نام `token` و زمان اعتبار تقریبی هفت ساعت می‌دهد.

پنل تصویر CAPTCHA را نمایش می‌دهد و عدد آن را کاربر وارد می‌کند. پاسخ CAPTCHA ذخیره نمی‌شود. کوکی‌های `token` و `cookiesession1` با AES-256-GCM و کلیدی خارج از پوشه داده رمزنگاری می‌شوند. قالب v2 نشست، فایل‌های رمزنگاری‌شدهٔ v1 (توکن‌محور) را نیز می‌خواند. پایش نشست در تمام ساعات شبانه‌روز فقط درخواست خواندنی `GET /orders` می‌فرستد؛ فاصلهٔ بررسی‌ها به‌صورت تصادفی بین ۱ تا ۷ دقیقه است. پایش، انقضای قطعی توکن را دور نمی‌زند: اگر دایا کوکی تازه یا اعتبار جدیدی صادر نکند، با پایان اعتبار نشست متوقف می‌شود و ورود دستی لازم خواهد بود. کوکی تازه تنها در صورت دریافت `Set-Cookie` معتبر ذخیره می‌شود و بدون شواهد، انقضای توکن تمدید نمی‌شود. پاسخ 401/403 به ورود دستی CAPTCHA نیاز دارد و پاسخ 429 پایش را متوقف می‌کند.

## وضعیت API سفارش دایا

**ارسال خودکار سفارش از API هنوز فعال نشده است.** تاکنون endpoint، ساختار payload، مقادیر enum خرید/فروش و اعتبار سفارش و ساختار پاسخ ثبت سفارش دایا از ترافیک واقعی تأیید نشده‌اند. برای جلوگیری از ارسال اشتباه یا تکراری، endpoint رابین به دایا تعمیم داده نشده و انتخاب مسیر API غیرفعال است.

تمام مدل سفارش، زمان‌بندی، برنامه‌ریز چندنوبتی، مدیریت وضعیت، تاریخچه، لغوها و مسیر دستی پیاده‌سازی شده‌اند. برای تکمیل ارسال خودکار، باید درخواست واقعی ثبت سفارش از Network مرورگر دایا (با حذف token، cookie، رمز، شماره حساب و شناسه‌های شخصی) دریافت شود. هیچ درخواست سفارش زنده‌ای در CI یا تست‌ها ارسال نمی‌شود.

## بررسی دستی endpointهای نشست در Preview

در صفحه «اتصال به دایا»، دکمه «بررسی دستی مسیرهای نشست» فقط با کلیک کاربر اجرا می‌شود و دقیقاً یک درخواست GET به هر یک از مسیرهای تأییدشدهٔ زیر می‌فرستد:

- `/orders`
- `/financials/portfolio/instant`

برای هر پاسخ، کد HTTP، زمان دریافت هدرهای پاسخ، و وجود/تعداد هدرهای `Set-Cookie` نمایش داده می‌شود. مقادیر کوکی یا توکن نه در گزارش و نه در لاگ نوشته نمی‌شوند. فقط کوکی‌های مجاز `token` و `cookiesession1` از پاسخ، با همان اعتبارسنجی موجود پردازش و در صورت چرخش، در ذخیره‌ساز رمزنگاری‌شده ثبت می‌شوند. redirect دنبال نمی‌شود و درخواست ناموفق باعث retry نمی‌شود.

این بررسی فقط در Preview در دسترس است، پس‌زمینه‌ای اجرا نمی‌شود و به‌تنهایی تضمین‌کننده تمدید نشست نیست؛ تمدید فقط در صورت صدور کوکی تازه از سمت دایا رخ می‌دهد.

## تست آنلاین در Preview

- مسیر `/daya-online-test` با کلیک کاربر فقط `GET /orders` واقعی را برای بررسی نشست و پاسخ JSON اجرا می‌کند.
- مسیر `/daya-online-test/one` فقط در Preview یک فرم ارسال دستی یک‌باره دارد. با تأیید صریح کاربر، همان یک درخواست واقعی `POST /orders` ارسال و نتیجه در تاریخچه Preview ذخیره می‌شود.
- سفارش آزمایشی وارد زمان‌بندی نمی‌شود. در timeout یا نتیجه مبهم، ارسال مجدد خودکار انجام نمی‌شود. در پاسخ ENTRY، فقط تأیید خواندنی BOOKED بررسی می‌شود و بعد از BOOKED پایش متوقف می‌ماند.
- هیچ درخواست POST واقعی در CI یا تست‌های خودکار اجرا نمی‌شود. ارسال زمان‌بندی‌شده از API همچنان به‌صورت مستقل غیرفعال است.

## توسعه محلی

Go 1.23 یا جدیدتر:

```bash
cp .env.example .env
make test
make vet
make build
make run
```

مقادیر واقعی `DAYA_ADMIN_USER`، `DAYA_ADMIN_PASS` و کلید رمزنگاری را در محیط امن قرار دهید. رمزها، توکن‌ها و کوکی‌ها نباید در Git ثبت شوند.

## Capturing the Daya order API contract

The authenticated order endpoint is not yet verified. To capture it without sharing raw credentials or account data, export a HAR locally and run:

```bash
python3 tools/sanitize_daya_har.py /path/to/network.har > daya-order-contract-sanitized.json
```

The sanitizer keeps endpoint paths, methods, field names/types, order enum values, response status and response shape, while redacting cookies, tokens, credentials, CAPTCHA values, account identifiers, order IDs, symbols, ISINs, prices and quantities. Review the resulting file before sharing it. Full instructions: [docs/capturing-order-contract.md](docs/capturing-order-contract.md).
