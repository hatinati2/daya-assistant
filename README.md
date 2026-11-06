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

| مورد | Production | Preview |
|---|---|---|
| پورت | 8100 | 8101 |
| سرویس | `daya-assistant.service` | `daya-assistant-preview.service` |
| پوشه برنامه | `/opt/daya-assistant` | `/opt/daya-assistant-preview` |
| داده‌ها | `/var/lib/daya-assistant` | `/var/lib/daya-assistant-preview` |
| تنظیمات | `/etc/daya-assistant.env` | `/etc/daya-assistant-preview.env` |
| کلید | `/etc/daya-assistant.key` | `/etc/daya-assistant-preview.key` |

اطلاعات ورود اولیه در فایل root-only ذخیره می‌شود: `/root/daya-assistant-credentials.txt` یا `/root/daya-assistant-preview-credentials.txt`. تنظیمات قبلی در به‌روزرسانی حفظ می‌شوند.

## احراز هویت دایا

قرارداد ورود از Network مرورگر استخراج شده است:
- `GET https://mg.dayatrader.ir/captcha`: تصویر PNG، هدر `captchaId` و کوکی `cookiesession1`.
- `POST https://mg.dayatrader.ir/oms-login`: فیلدهای `username`، `password`، `captchaAnswer` و `captchaId` همراه هدر `x-client-type: WEB_CLIENT` و همان کوکی CAPTCHA.
- پاسخ موفق، کوکی HttpOnly با نام `token` و زمان اعتبار تقریبی هفت ساعت می‌دهد.

پنل تصویر CAPTCHA را نمایش می‌دهد و عدد آن را کاربر وارد می‌کند. پاسخ CAPTCHA ذخیره نمی‌شود. کوکی نشست با AES-256-GCM و کلیدی خارج از پوشه داده رمزنگاری می‌شود؛ نشست منقضی‌شده فعال تلقی نمی‌شود.

## وضعیت API سفارش دایا

**ارسال خودکار سفارش از API هنوز فعال نشده است.** تاکنون endpoint، ساختار payload، مقادیر enum خرید/فروش و اعتبار سفارش و ساختار پاسخ ثبت سفارش دایا از ترافیک واقعی تأیید نشده‌اند. برای جلوگیری از ارسال اشتباه یا تکراری، endpoint رابین به دایا تعمیم داده نشده و انتخاب مسیر API غیرفعال است.

تمام مدل سفارش، زمان‌بندی، برنامه‌ریز چندنوبتی، مدیریت وضعیت، تاریخچه، لغوها و مسیر دستی پیاده‌سازی شده‌اند. برای تکمیل ارسال خودکار، باید درخواست واقعی ثبت سفارش از Network مرورگر دایا (با حذف token، cookie، رمز، شماره حساب و شناسه‌های شخصی) دریافت شود. هیچ درخواست سفارش زنده‌ای در CI یا تست‌ها ارسال نمی‌شود.

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
