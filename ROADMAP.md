# CacheStorm geliştirme yol haritası

20 Ekim 2026 incelemesinde öncelik, mevcut komutların veri güvenliğini ve gerçek çalışma davranışını tamamlamak olarak belirlendi. Ana modülün testleri ve yarış denetimi geçiyor; buna rağmen transaction kalıcılığı, dağıtık çalışma bağlantıları ve SDK davranışlarında çalıştırılarak doğrulanmış eksikler var. Ayrıntılar [inceleme raporunda](docs/PROJECT_REVIEW.md).

Bu planın başlangıç noktası `fe598ff` ve çalışma ağacındaki mevcut değişikliklerdir. Süreler tek geliştirici için yaklaşık iş günü tahminidir; yayın tarihi taahhüdü değildir. Yeni komut, GraphQL veya bulut entegrasyonu ekleme işi ilk dört kilometre taşının çıkış ölçütlerinden sonra ele alınmalı.

## Öncelik sırası

| Öncelik | İş | Tamamlanma ölçütü | Bağımlılık |
|---------|----|------------------|------------|
| P0 | Transaction yazmalarının AOF kaydı | Başarılı EXEC sonrasında yeniden açılan sunucunun verisi aynı; yarım transaction tekrar oynatılmıyor | Yok |
| P0 | EXEC komut davranışı ve atomiklik | Desteklenen komutlar transaction içinde aynı sonucu veriyor; başka bağlantı ara durumu göremiyor | AOF tasarımıyla birlikte |
| P0 | Gerçek destek kapsamını açıklamak | Cluster, replika, SELECT, SDK, kalıcılık ve lisans açıklamaları kodla tutarlı | Yok |
| P1 | Redis uyumluluk matrisi | Seçilen Redis sürümüne karşı otomatik karşılaştırma; hata, TTL, binary veri ve protokol tipleri ölçülüyor | Komut envanteri |
| P1 | SDK'ları çalışır ve test edilir hale getirmek | Üç SDK için gerçek taşıma, hata, yaşam döngüsü ve paketleme kontrolleri CI'da | Sunucunun temel sözleşmeleri |
| P1 | CI ve örnek program kapsamı | Go ana modülü, beş Go örneği, SDK'lar, web derlemesi ve doküman bağlantıları zorunlu kontrol | SDK test altyapısı |
| P2 | Cluster ve replikasyonun sunucuya bağlanması | Gerçek çok düğümlü senaryoda veri eşitliği, yeniden katılım ve failover doğrulanıyor | Kalıcılık ve komut metadata'sı |
| P2 | Operasyon ve performans | Konfigürasyon, izleme ve kapasite ölçümleri tekrar üretilebilir | Önceki doğruluk kapıları |

## M0 Depo ve belge temizliği

Durum: **bu incelemede tamamlandı**.

- [x] Sekiz eski tasarım belgesi içerikleri korunarak `docs/archive/design/` altına taşındı.
- [x] Kullanılmayan GoReleaser yapılandırması arşivlendi.
- [x] Test yedeği ve referansı olmayan üç web şablon görseli kaldırıldı; yerel yedekleri korundu.
- [x] On dört kırık belge bağlantısı düzeltildi.
- [x] Kapsam raporu gerçek ölçümle yenilendi; katkı ve örnek rehberlerindeki eski iddialar azaltıldı.
- [x] Python/Node çıktıları Git dışına; web bağımlılıkları ve denetim çıktıları Docker bağlamı dışına alındı.
- [x] Ana Docker sağlık kontrolü `/api/health` adresine düzeltildi ve doğrulandı.

Önceden değiştirilmiş README ve iki komut belgesi korunmuştur. Bu dosyalardaki eski iddialar, dört workflow bağlantısı ve 14 başlık bağlantısının giderilmesi M1 kapsamındadır. Arşiv ve saklama kararları raporda bulunur.

## M1 Veri güvenliği ve transaction

Durum (25 Ekim 2026): **tamamlandı.** EXEC kuyruğu kayıtlı handler'larla çalışıyor (F22), başarılı EXEC AOF'a tek `MULTI ... EXEC` bloğu olarak yazılıyor ve yarım blok replay'de atlanıyor (F20). Yarım kalmış AOF kuyruğu kırpılıyor, bozuk AOF başlangıcı durduruyor. Transaction ve Lua script'leri artık diğer istemcilerden izole: okuyucular yarım uygulanmış bir bloğu göremiyor. Gerçek 16 veritabanı ve veritabanı başına WATCH uygulandı.

Öncelik: **P0**. Tahmin: **5–10 iş günü**.

1. F20 için EXEC sonuçlarını ve AOF kaydını tek işlem sınırına bağla. Kuyruğu yalnızca uygulayıp EXEC'i kaydetmemek, başarılı yazmaları kaybettiriyor.
2. Transaction tekrar oynatmasını bir bütün olarak tanımla; kısmi dosya, kesintili yazma ve açılışta replay hatası durumlarını test et. AOF hata verdiğinde hangi yanıtın ve hangi yazma durumunun geçerli olduğunu belirle.
3. F22 için ayrı `executeQueuedCommand` switch'ini ortak komut yürütme yolu ile değiştir. Auth, ACL, hata biçimi ve kalıcılık kurallarının tekrar uygulanmasını veya atlanmasını önle.
4. Transaction atomikliğini bariyerlerle sınayan çok bağlantılı testler yaz. WATCH için oluşturma, silme, yeniden oluşturma, expiry ve başarısız yazma senaryolarını doğrula.
5. Yazan komut listesini, Lua/FCALL ve HTTP üzerinden yazmaları karşılaştır. Relative TTL'nin replay sırasında yeniden başlamasını ve rastgele komutların farklı sonuç vermesini hedefli testlerle araştır.
6. Destek beyanlarını düzelt: 1.596 kayıtlı komut tam uygulama anlamına gelmiyor. SELECT tek keyspace kullanıyor; cluster ve replika yapılandırması bağlı değil. README, web ve paket metadata'sındaki lisans metinlerini kök LICENSE ile birlikte uzlaştır.

Çıkış ölçütü: F20 ve F22'nin değişmeyen kanıtları geçer; yeni kalıcı regresyonlar transaction/replay davranışını korur; hedefli testler, paket testleri, build/vet ve yarış denetimi başarılıdır. RDB snapshot ve AOF fsync işlemlerinin farklı garantileri belgelenmiştir.

## M2 Redis sözleşmeleri ve komut envanteri

Durum (25 Ekim 2026): **büyük ölçüde tamamlandı.** `tests/compat` Redis 7.4'e karşı 340 vakada farkları ölçüyor: 337 eşleşiyor, kalan 3 fark [raporda](tests/compat/REPORT.md). Komut metadata'sı (arity, bayraklar, anahtar konumları) Redis'in kendi COMMAND çıktısından `tools/cmdspecgen` ile üretiliyor ve router, ACL anahtar çıkarımı, OOM kontrolü bu tabloyu kullanıyor. RESP2 varsayılan; HELLO 2 destekleniyor, RESP3 `NOPROTO` ile reddediliyor. Açık: RESP3, 2262 sonrası expiry anları, gopher-lua hata metinleri.

Öncelik: **P1**. Tahmin: **5–10 iş günü**. M1 ile ortak komut yürütme tasarımını kullanır.

1. Hedef Redis sürümünü ve desteklenen protokol alt kümesini açıkça seç. `tam`, `kısmi`, `desteklenmiyor` ve `CacheStorm uzantısı` ayrımlarını komut/alt komut bazında tut.
2. Komut metadata'sında arity, anahtar konumları, read/write/admin özellikleri ve scripting/transaction kurallarını tanımla. Router, ACL, COMMAND çıktısı ve kalıcılık aynı veriyi kullansın.
3. Komut sayısını üretim router'ından üret; belgeler, web ve testlerde ayrı elle yazılan sayılar tutma.
4. Önce string/hash/list/set/zset, TTL, SCAN, transaction, stream, scripting ve Pub/Sub için gerçek Redis ile karşılaştırmalı testler ekle. Aynı veri ve aynı komut dizisi aynı yanıt ve son durumu üretmeli.
5. RESP için parçalanmış/birleşmiş çerçeveler, binary ve UTF-8, null, yanlış uzunluk, iç içe veri ve boyut sınırlarını fuzz testlerine ekle. Desteklenmeyen RESP3 tiplerini açıkça belirt.
6. SELECT/FLUSHDB/DBSIZE için çoklu veritabanı desteği uygulanacak mı, yalnızca DB 0 mı desteklenecek kararını ver; sessizce paylaşılan keyspace davranışını belgeyle gizleme.

Çıkış ölçütü: uyumluluk raporu hedef sürümü, test edilen komut sayısını, başarısız ve atlanan vakaları birlikte gösterir. Kaynağı olmayan “%99 uyum” iddiası yerine bu rapor kullanılır.

## M3 SDK doğruluğu ve dağıtımı

Durum (25 Ekim 2026): **tamamlandı.** F23–F27 düzeltildi ve kalıcı testlere alındı (Python 80, TypeScript 32, Go 16 test). Python `pyproject.toml` ile wheel/sdist üretiyor; TypeScript gerçek devDependency'ler ve lockfile kullanıyor. Ayrıca go-redis v9, redis-py 5 ve ioredis 5 `scripts/client-smoke.sh` ile CI'da sınanıyor. Açık: otomatik yeniden bağlanma, paketlerin npm/PyPI'ye yayımlanması.

Öncelik: **P1**. Tahmin: **8–15 iş günü**. Bağımsız codec işleri M1 sürerken başlayabilir.

| İstemci | Yapılacak iş | Kabul testi |
|---------|-------------|------------|
| Python | F23 binary güvenli encoder; F26 async bağlantı timeout'u; havuzun auth/DB ve kapanış davranışı | Türkçe/emoji/binary veri aynı kalır; async bağlantı kurulur; kopma, timeout ve auth hataları görünür |
| TypeScript | F24 gerçek pipeline; F25 birikimli RESP decoder ve byte uzunlukları; hata ve timeout temizliği | Pipeline gerçekten yazar; parçalı/birleşik yanıtlar doğru eşleşir; hata yanıtı promise'i reddeder |
| Go | F27 kapanış sonrası hata; Pub/Sub receive; gerçek pipeline batching; havuz sınırları ve context | Close sonrası panik yok; mesaj alınır; iptal/kopma sırasında bağlantı ve goroutine sızıntısı yok |
| Üçü | Pub/Sub, yeniden bağlanma ve desteklenen özellikler için açık sözleşme | İlan edilen her özellik için transport ve sonuç doğrulayan test |

Python için `pyproject.toml`, desteklenen Python sürümleri, wheel/sdist ve kurulum smoke testi ekle. TypeScript için gerçekten kullanılan test/lint araçlarını tanımla ve lockfile politikasını belirle. Go SDK için paket testleri ve dış kullanıcı import smoke testi ekle. Paketlerin lisans alanlarını yayın öncesi uzlaştır.

Çıkış ölçütü: F23–F27 geçer ve kalıcı testlere alınmıştır; yerel olarak paketlenen SDK üç dilde de yeni açılan, auth açık sunucuya bağlanıp SET/GET, hata, pipeline ve ilan edilen Pub/Sub senaryosunu tamamlar.

## M4 CI ve yayın güvenilirliği

Durum (25 Ekim 2026): **tamamlandı.** CI; Python, TypeScript ve Go istemcilerini, beş örnek modülü (readonly), Redis uyumluluk düzeneğini (ratchet modunda), gerçek istemci duman testini ve Docker imaj duman testini (non-root, sağlık, requirepass, protected mode, sürüm) çalıştırıyor. Release imajı sürüm damgasıyla ve alpine 3.22 üzerinde üretiliyor; release action'ı Node 16'dan çıkarıldı.

Öncelik: **P1**. Tahmin: **3–5 iş günü**; M2/M3 ile kademeli uygulanır.

1. Go ve linter sürümlerini geliştirme rehberi ve CI arasında sabitle. Üç yerel staticcheck bulgusunu mevcut değişikliklerin sahibiyle birlikte gider; denetimi kapatarak geçirme.
2. `internal/...` testlerine ek olarak kök `./...`, plugin'ler ve SDK testlerini kontrol et. Gerçek entegrasyon sunucusunu CI başlatsın; zorunlu senaryolar atlandığında kontrol başarısız olsun.
3. Beş Go örneğinin modül sürümünü/sum dosyalarını düzelt; readonly build'lerini CI'a ekle. Python ve TypeScript örnekleri yayımlanmış farklı sürümler yerine amaçlanan paket çıktısını kullansın.
4. Web build'ini pull request kontrolüne ekle. Depo içi dosya/anchor bağlantıları ve komut dokümanı üretimi sapmasını kontrol et.
5. Docker imajını build et, non-root kullanıcıyla başlat, sağlık/readiness ve auth kontrolünü çalıştır. Release testi ve normal CI aynı kalite kapısına dayansın.
6. Release arşivlerinin lisans ve örnek config içeriğini, sürüm/build zamanını ve checksum doğrulamasını sınayan smoke testi ekle. Çalışmayan paket yöneticisi/yayın vaatlerini belgelerden kaldır veya ayrı workflow olarak uygula.

Çıkış ölçütü: bir PR yalnızca Go birim testlerinin geçmesiyle yeşil görünmez; örnekler, SDK'lar, web ve Docker da sonuç üretir. Yayın dosyaları test edilen aynı commit'ten üretilir.

## M5 Gerçek cluster ve replikasyon

Öncelik: **P2**, dağıtık kullanım hedefleniyorsa **P0 ürün engeli**. Tahmin: **15–30 iş günü**, tasarıma göre yeniden değerlendirilir.

Durum (25 Ekim 2026): 1. ve 3. maddeler tamamlandı (tek sahip `internal/replication.Manager`; iki/üç sunuculu uçtan uca testler ve `scripts/replication-smoke.sh`). Cluster bu sürümde desteklenmiyor: `cluster.enabled=true` başlangıçta hata veriyor, CLUSTER komutları "cluster support disabled" döndürüyor (2. ve 4. maddelerin "desteklenmiyor hatası" seçeneği). 5. madde ve Sentinel ile otomatik failover açık.

1. F21 için command katmanındaki replication manager ile `internal/replication.Manager` arasındaki sahipliği birleştir. Config, Start, Stop ve rol değişimleri aynı örneğe ulaşsın.
2. Cluster, gossip, hash-slot routing ve failover yaşam döngülerini Server'a bağla. Etkinleştirilen ama başlatılamayan özellik açık hata versin.
3. REPLICAOF/SYNC/PSYNC/backlog/ACK ve yazma yayılımını iki gerçek süreç üzerinde doğrula. Replica read-only, yeniden bağlantı ve ilk senkronizasyonu sınayan testler ekle.
4. CLUSTER DELSLOTS/SETSLOT/COUNTKEYSINSLOT/GETKEYSINSLOT, MOVED/ASK/CROSSSLOT ve MIGRATE için tamamlanmamış davranışları uygula veya desteklenmiyor hatası döndür.
5. Üç düğümde node kaybı, ağ bölünmesi, yeniden katılım, eski primary'nin dönüşü ve slot taşımayı veri eşitliğiyle doğrula. Sentinel'in gerçek rol değişimini tetiklediğini kontrol et.

Çıkış ölçütü: sadece paket testleri değil, süreçlerin açılıp kapandığı tekrarlanabilir bir kümede veri kaybı/çakışma koşulları ve destek sınırları ölçülür. Bu ölçüt sağlanmadan dağıtık production desteği ilan edilmez.

## M6 Operasyon, performans ve kapsam daraltma

Durum (25 Ekim 2026): **kısmen tamamlandı.** `maxmemory` artık yerinde büyüyen koleksiyonlar için de çalışıyor ve `denyoom` komutları sınırı aşınca OOM alıyor. Performans ölçümleri yöntemleriyle [docs/BENCHMARKS.md](docs/BENCHMARKS.md) içinde. Protected mode, sentinel'in isteğe bağlı olması, ortam değişkeniyle parola, kapanışta hızlı drain ve accept backoff eklendi. Açık: `internal/command` içindeki uzantı komutlarının çekirdek/opsiyonel ayrımı, RDB snapshot'ının sunucuya bağlanması.

Öncelik: **P2**. Tahmin: **5–10 iş günü**; ölçüm sonuçlarına göre bölünür.

- `config/example.yaml` ile geçerli şemayı birleştir; önce Makefile ve installer referanslarını taşı. Yoksayılan `plugins.stats`, HTTP CORS/rate ayarları, buffer/keepalive ve snapshot ayarlarını uygula ya da şemadan/belgeden kaldır.
- Root Dockerfile ile kullanılan diğer Dockerfile'ları, tekrar üretilebilir build hedefi korunarak ortaklaştır. Cluster Compose örneğinin okunmayan ortam değişkenlerini düzelt.
- Prometheus/Grafana kurulumunu auth açık sunucuyla doğrula; alert dosyası mount'u, endpoint ve kimlik bilgileri çalışsın.
- Config ve logger başlangıç hatalarını, kapanışı, disk dolmasını, yavaş istemciyi ve uzun süreli bellek kullanımını sınayan işletim testleri ekle.
- Sabit veri kümesi ve worker/pipeline ayarlarıyla throughput, p50/p95/p99, RSS, allocation ve GC ölç. Store mikrobenchmark'ı ile TCP uçtan uca ölçümü ayrı göster; regresyon eşiklerini ilk güvenilir baseline'dan belirle.
- `internal/command` içindeki yaklaşık 49 bin satırlık uygulamayı, ölçüm ve kullanım verisine göre çekirdek/opsiyonel uzantılara bölmeyi değerlendir. Kullanılmayan komut ailesini kanıt ve geçiş planı olmadan silme.

Çıkış ölçütü: kurulum ve izleme belgelerindeki adımlar temiz ortamda çalışır; performans rakamlarının komutu, ortamı ve ham sonucu vardır. Kritik davranışların testleri tamamlandıktan sonra ölçülmüş kapsam için aşamalı hedef belirlenir.

## Bir sonraki somut değişiklik

M1–M4 tamamlandı; replikasyon çalışıyor. Sıradaki işler:

1. RDB snapshot'ını (SAVE/BGSAVE ve başlangıçta yükleme) sunucuya bağlamak; tam senkron için fork'suz anlık görüntünün yazma duraklamasını ölçüp azaltmak.
2. Expiry'yi milisaniye olarak saklayıp 2262 sınırını kaldırmak.
3. Sentinel ile otomatik failover'ı gerçek süreçlerle doğrulamak.
4. RESP3 (HELLO 3, push mesajları, map/set/double tipleri).
5. Pipeline'lı yazmalarda yazma kapısının maliyetini düşürmek (Redis'in %40–85'i).
