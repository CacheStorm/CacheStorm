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

Durum (25 Ekim 2026): **büyük ölçüde tamamlandı.** `tests/compat` Redis 7.4'e karşı 346 vakada farkları ölçüyor: 345 eşleşiyor, kalan Lua hata metni farkı [raporda](tests/compat/REPORT.md). Komut metadata'sı (arity, bayraklar, anahtar konumları) Redis'in kendi COMMAND çıktısından `tools/cmdspecgen` ile üretiliyor ve router, ACL anahtar çıkarımı, OOM kontrolü bu tabloyu kullanıyor. RESP2 varsayılan; HELLO 2 destekleniyor, RESP3 `NOPROTO` ile reddediliyor. Expiry milisaniye olarak saklanıyor; 2262 sınırı kaldırıldı. Açık: RESP3, gopher-lua hata metinleri.

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

Durum (25 Ekim 2026): 1. ve 3. maddeler tamamlandı (tek sahip `internal/replication.Manager`; iki/üç sunuculu uçtan uca testler ve `scripts/replication-smoke.sh`). Cluster bu sürümde desteklenmiyor: `cluster.enabled=true` başlangıçta hata veriyor, CLUSTER komutları "cluster support disabled" döndürüyor (2. ve 4. maddelerin "desteklenmiyor hatası" seçeneği). Tam senkronizasyon function kitaplıklarını (boş kayıt dahil) taşıyor; canlı LOAD/DELETE ve replikanın SIGKILL sonrası AOF kurtarması iki ayrı süreçle doğrulandı. Function kayıtları sunucuya ait; hata yanıtı veren snapshot komutları ve başarısız AOF kaydı senkronizasyonu durduruyor, bu konum devam edilebilir olarak kabul edilmiyor. Başarısız tam snapshot artık canlı veriyi değiştirmeden reddediliyor; yeni veri ayrı alanda hazırlanıp AOF adımından sonra yayımlanıyor. Canlı komut hatası konumu ilerletmiyor, bağlantıyı sonlandırıyor ve tam senkronizasyon gerektiriyor. Canlı komutun hata öncesi kısmi bellek değişikliklerinin ele alınması, 5. madde ve Sentinel ile otomatik failover açık.

1. F21 için command katmanındaki replication manager ile `internal/replication.Manager` arasındaki sahipliği birleştir. Config, Start, Stop ve rol değişimleri aynı örneğe ulaşsın.
2. Cluster, gossip, hash-slot routing ve failover yaşam döngülerini Server'a bağla. Etkinleştirilen ama başlatılamayan özellik açık hata versin.
3. REPLICAOF/SYNC/PSYNC/backlog/ACK ve yazma yayılımını iki gerçek süreç üzerinde doğrula. Replica read-only, yeniden bağlantı ve ilk senkronizasyonu sınayan testler ekle.
4. CLUSTER DELSLOTS/SETSLOT/COUNTKEYSINSLOT/GETKEYSINSLOT, MOVED/ASK/CROSSSLOT ve MIGRATE için tamamlanmamış davranışları uygula veya desteklenmiyor hatası döndür.
5. Üç düğümde node kaybı, ağ bölünmesi, yeniden katılım, eski primary'nin dönüşü ve slot taşımayı veri eşitliğiyle doğrula. Sentinel'in gerçek rol değişimini tetiklediğini kontrol et.

Çıkış ölçütü: sadece paket testleri değil, süreçlerin açılıp kapandığı tekrarlanabilir bir kümede veri kaybı/çakışma koşulları ve destek sınırları ölçülür. Bu ölçüt sağlanmadan dağıtık production desteği ilan edilmez.

## M6 Operasyon, performans ve kapsam daraltma

Durum (25 Ekim 2026): **kısmen tamamlandı.** `maxmemory` artık yerinde büyüyen koleksiyonlar için de çalışıyor ve `denyoom` komutları sınırı aşınca OOM alıyor. Performans ölçümleri yöntemleriyle [docs/BENCHMARKS.md](docs/BENCHMARKS.md) içinde. Protected mode, sentinel'in isteğe bağlı olması, ortam değişkeniyle parola, kapanışta hızlı drain ve accept backoff eklendi. SAVE/BGSAVE, periyodik snapshot ve başlangıçta geri yükleme sunucuya bağlandı; çoklu DB, TTL, etiket, stream consumer-group ve function kitaplıkları korunuyor. Açık: `internal/command` içindeki uzantı komutlarının çekirdek/opsiyonel ayrımı ve snapshot yakalama maliyetinin ölçülmesi.

Öncelik: **P2**. Tahmin: **5–10 iş günü**; ölçüm sonuçlarına göre bölünür.

- [x] (27 Ekim 2026) Eski şemalı `config/example.yaml` kaldırıldı; Makefile ve iki installer geçerli şablona ve geçerli şemaya taşındı. Loader bilinmeyen anahtarı dosya/satır bilgisiyle reddediyor; `config/*.yaml` bir testle yüklenebilir tutuluyor. `tcp_keepalive`, buffer boyutları uygulanıyor; `namespaces` ve `plugins.stats/auth` şemadan çıkarıldı; `max_aof_size` yerine Redis karşılığı `aof_rewrite_percentage`/`aof_rewrite_min_size` geldi.
- Root Dockerfile ile kullanılan diğer Dockerfile'ları, tekrar üretilebilir build hedefi korunarak ortaklaştır. Cluster Compose örneğinin okunmayan ortam değişkenlerini düzelt.
- [x] (27 Ekim 2026) Prometheus kimlik gerektirmeyen metrik portunu (`plugins.metrics`) kazıyor; iki uç aynı, tüm veritabanlarını kapsayan çıktıyı veriyor. `alerts.yml` Compose'a bağlandı; tetiklenemeyen `cachestorm_up == 0` alarmı `up{job="cachestorm"} == 0` oldu.
- Config ve logger başlangıç hatalarını, kapanışı, disk dolmasını, yavaş istemciyi ve uzun süreli bellek kullanımını sınayan işletim testleri ekle.
- Sabit veri kümesi ve worker/pipeline ayarlarıyla throughput, p50/p95/p99, RSS, allocation ve GC ölç. Store mikrobenchmark'ı ile TCP uçtan uca ölçümü ayrı göster; regresyon eşiklerini ilk güvenilir baseline'dan belirle.
- `internal/command` içindeki yaklaşık 49 bin satırlık uygulamayı, ölçüm ve kullanım verisine göre çekirdek/opsiyonel uzantılara bölmeyi değerlendir. Kullanılmayan komut ailesini kanıt ve geçiş planı olmadan silme.

Çıkış ölçütü: kurulum ve izleme belgelerindeki adımlar temiz ortamda çalışır; performans rakamlarının komutu, ortamı ve ham sonucu vardır. Kritik davranışların testleri tamamlandıktan sonra ölçülmüş kapsam için aşamalı hedef belirlenir.

## Bir sonraki somut değişiklik

27 Ekim 2026 turu: `BGREWRITEAOF` artık gerçekten AOF'u yeniden yazıyor (önceden yalnızca "started" dönüyordu) ve AOF otomatik olarak küçültülüyor. `CONFIG GET/SET` çalışan sunucuya bağlandı: `maxmemory`, `maxmemory-policy`, `maxmemory-samples`, `maxclients`, `timeout` ve `requirepass` anında etkili; `CONFIG GET` Redis gibi glob eşleşiyor ve çoklu parametre alıyor. Sekiz Redis eviction politikasının hepsi var (volatile-lfu/random/ttl eklendi). Boşta bekleyen istemciler artık 5 dakikada kesilmiyor (Redis `timeout 0`). Lisans MIT.

M1–M4 tamamlandı; replikasyon çalışıyor. Sıradaki işler:

1. SAVE/BGSAVE, periyodik snapshot ve başlangıçta yükleme bağlandı. Sırada native snapshot yakalama ile tam senkronun yazma duraklamasını ve ek bellek maliyetini ölçüp azaltmak var.
2. Tamamlandı (25 Ekim 2026): expiry milisaniye olarak saklanıyor; 2262 sınırı kaldırıldı. AOF/RDB roundtrip ve Redis karşılaştırmasıyla doğrulandı.
3. Tamamlandı (27 Ekim 2026): gerçek Redis Sentinel ile otomatik failover üç CacheStorm süreciyle doğrulandı ve CI'a alındı (`scripts/sentinel-smoke.sh`). Yerleşik sentinel rolü deneysel olarak işaretlendi; sunucuları yeniden yapılandırmıyor.
4. Tamamlandı (27 Ekim 2026): RESP3 (HELLO 3, push, map, double, null) ve gerçek CLIENT TRACKING (client-side caching). go-redis, redis-py (client-side cache dahil) ve ioredis CI'da.
5. Pipeline'lı yazmalarda yazma kapısının maliyetini düşürmek (Redis'in %40–85'i).

AOF snapshot değişimi (25 Ekim 2026): yazarı durdurup yeniden açma kaldırıldı; eşzamanlı append kilit altında sıralanıyor. Hazırlama, yazma, fsync, rename ve directory-sync hataları kalıcı regresyonlarla sınanıyor. Geri alma sonrası append, hard-link bulunmayan dosya sisteminde de doğru dosyaya gidiyor. Geri alma da başarısızsa önceki veri korunuyor, kurtarma yolu bildiriliyor ve writer hata döndürüyor. AOF hataları artık MISCONF/HTTP 503 olarak istemciye taşınıyor; sonraki AOF kapsamındaki yazmalar uygulama öncesi reddediliyor. INFO ve readiness hatayı gösteriyor. Açık: ilk hatalı işlemin bellek/disk belirsizliği, güç kesintisi kurtarması ve Windows çalışma zamanı doğrulaması.

Replika checkpoint (25 Ekim 2026): sürümlü/özet korumalı metadata, AOF boyutu/SHA-256 ve kaynak doğrulaması eklendi. AOF flush/fsync/close başarılı olmadan checkpoint yayımlanmıyor; dosya ve POSIX dizin senkronizasyonu doğrulanıyor. Başlangıç marker silmesini kalıcılaştırmadan yazmalara açılmıyor. Değişmiş/eksik/yarım AOF ve eski/bozuk metadata tam senkron gerektiriyor. Yazılabilir replikadaki yerel komut/EXEC/script/REST yazmaları devam geçmişini geçersiz kılıyor; reconnect/restart tam senkron yapıyor, promotion eski kısmi geçmişi bırakmıyor. Hata enjeksiyonu ve süreç testleri eklendi. Açık: gerçek güç kesintisi, donanım/dosya sistemi hata modeli, Windows dizin dayanıklılığı ve ortak/ağ dosya sistemi semantiği.

AOF replay (25 Ekim 2026): bilinmeyen/izin verilmeyen komut, arity ve handler hata yanıtı artık başlangıcı ilk hatada durduruyor. Geçersiz tamamlanmış log değiştirilmiyor. Transaction framing doğrulanıyor; yarım son transaction MULTI sınırından kırpılıp fsync ediliyor. Böylece sonraki onaylı yazmanın eski yarım bloğa girip kaybolması engellendi. Transaction'ın her byte kesimi, DB seçimi, DISCARD ve süreç SIGKILL/restart regresyonları eklendi; süreç testi CI'a bağlandı. Açık: tüm komutların deterministik replay'i, expiry/eviction etkileşimleri, süreç geneli uzantı kayıtlarının başarısız yükleme sonrası geri alınması ve büyük AOF yükleme maliyeti.

Tam doğrulamada bulunan ek yarışlar (25 Ekim 2026): FIFO bekleyicinin kayıt/uyandırma metadata aralığında anahtar kaybı, readiness sonrası sahiplik kaybı ve aktif sahibin önüne geçme yeniden üretildi. Kayıt atomikleşti; retry/sıra bırakma token kimliğiyle yapılıyor. Go istemcisinde socket deadline'ın context timer Err kaydından önce dolması için DeadlineExceeded eşlemesi düzeltildi. Bu kanıtlar kalıcı regresyon testlerine eklendi.

Veri dizini sahipliği (25 Ekim 2026): Server.New, AOF/RDB okumadan önce OS seviyesinde tek sahip kilidi alıyor; göreli yol ve symlink aynı kilide gidiyor. Çözülmüş dizin sunucu ömrü boyunca sabit. Kurucu/başlangıç hatası ve tamamlanmış Stop kilidi bırakıyor; bitmeyen worker varsa Stop hata dönüp sahipliği tutuyor. Tekrarlı/eşzamanlı Stop güvenli; başarısız kalıcı Start sonrası New gerekiyor. Süreç testi ikinci sahibin AOF/RDB/checkpoint'i değiştiremediğini ve SIGTERM/SIGKILL sonrası yeniden açılabildiğini doğruluyor; CI'a eklendi. Linux çalışma zamanı ve yedi yayın hedefinin cross-build'i doğrulandı. Açık: diğer OS'lerde native çalışma zamanı, ortak/ağ dosya sistemi kilit semantiği, süreç dışı dosya müdahaleleri ve güç kesintisi modeli.

Kapanış sahipliği doğrulaması: son kabul edilen bağlantı kaydolmadan accept goroutine'inin geride kalabildiği ve HTTP Shutdown timeout'unun yutulup sahipliğin erken bırakıldığı iki durum yeniden üretildi. Stop artık accept döngüsünü join ediyor; HTTP/TCP drain tamamlanmadıysa hata dönüyor, AOF yazarı ve dizin kilidi açık kalıyor. Gecikmiş HTTP yazmasının hâlâ AOF'a gittiği kalıcı regresyonla doğrulanıyor.

TTL geçmişi (25 Ekim 2026): core SET/expiry ve REST yazmaları mutlak son tarih ve etkili mutasyon olarak kaydediliyor. AOF v2 replay sırasında expiry/eviction erteleniyor; gerçek silinmeler DEL ile sıralanıyor. Replika, gecikmiş PERSIST/kısmi yeniden başlatma için süresi geçmiş fiziksel veriyi okumalardan gizleyerek tutuyor. Header'sız AOF mevcut kurallarıyla yüklenip atomik snapshot dönüşümü yapılıyor; eski logun eksik zaman bilgisi geri kazanılamaz ve eski binary v2 dosyasını okuyamaz. Açık: tüm uzantılar/rastgele işlemler/kaynak sınırlarında deterministik replay, güç kesintisi ve farklı OS çalışma zamanı doğrulaması. %100 production-ready iddiası yok.

DUMP/RESTORE (25 Ekim 2026): string/list/set/hash/zset için Redis ikili payload, CRC64/sürüm/yapı doğrulaması ve çift yönlü gerçek Redis aktarımı eklendi. Göreli TTL varsayılanı, ABSTTL, seçenek/hata davranışı, mutlak AOF yayılımı, transaction/script ve gecikmeli replika doğrulandı. Eski özel delimiter formatı yalnızca AOF kurtarmasında okunuyor; eski istemci dump yedekleri eşleşen binary ile yükseltme öncesi dışa aktarılmalı. 19 yeni corpus senaryosuyla 365 karşılaştırmada 363 tam eşleşme ölçüldü; iki belgelenmiş fark ayrı raporlanıyor. İki belgelenmiş fark: Lua hata metni ve int64 göreli RESTORE TTL taşmasının reddi. İkinci farkın Redis 7.4.11'de OK sonrası negatif ABSTTL/AOF replay veri kaybı ürettiği ayrı süreçte kanıtlandı. Açık: stream/group/PEL ve hash alan expiry'si dahil tüm Redis dump tipleri, modül/uzantı payload'ları, çok büyük LRU metadata ve kaynak sınırlarının tam semantiği. Bu tur native RDB dosya uyumluluğu sağlamıyor; %100 production-ready hedefi açık.

Bitmap eşzamanlılığı: SETBIT/BITFIELD ile bit okuyucuları arasındaki data race gerçek testte yeniden üretildi. Önceki kilitsiz DUMP okumasını geri getiren tek satırlık Go overlay de DUMP/yazma yarışını doğruladı. Bitmap değerine RWMutex eklendi; okuyucular kilit alıyor, string/DUMP aktarımı ve BITOP kaynakları korunan kopya kullanıyor. String'den dönüşüm eski immutable baytları değiştirmiyor ve TTL/etiketleri koruyor. İlk oluşturma/dönüşüm ve BITOP Store.Set hatalarının başarı yanıtıyla yutulduğu altı durum kanıtlandı ve düzeltildi. Açık: bitmap'in yerinde büyümesinde tam bellek kabulü ve eşzamanlı komutlar için geniş yük ölçümleri.

Bitmap büyütme (25 Ekim 2026): SETBIT/BITFIELD önce doğrulanıp planlanıyor; veri tamponunu büyütmeden paylaşılan izlenen bellek bütçesinden atomik pay ayrılıyor. Noeviction reddi mevcut baytları, TTL/etiketleri, WATCH/AOF ve muhasebeyi koruyor; EXEC/Lua ve maksimum offset süreç testi eklendi. Pay rezervasyonu sonrası yük tek kez muhasebeleşiyor. Yerel eviction sonrası başarısız yazmanın replica checkpoint'ini yanlışlıkla geçerli bıraktığı kanıtlandı ve düzeltildi; full resync ile yeniden uygunluk doğrulandı. BITFIELD_RO, #offset, SET overflow ve tam sözdizimi doğrulaması eklendi. Önceki uppercase/u64 Redis varsayımları gerçek 7.4.11 ölçümüyle düzeltildi. Yeni AOF etkileri çözülmüş SET alt işlemlerine dönüşüyor; yaygın eski kayıtlar kurtarma sırasında eski davranışlarıyla okunuyor. 16 sabit yeni diferansiyel senaryo ve 80 deterministik üretilmiş sıra eklendi. Açık: diğer mutasyonların rezervasyon modeline geçirilmesi, tüm süreç RAM'i/geçici tamponlar, DB'ler arası eviction seçimi, çok büyük değerlerin tüm kalıcılık yolları, uzantılar ve %100 kapsam hedefi.

HyperLogLog kalıcılığı (25 Ekim 2026): SAVE'in HLL değerini reddettiği ve full sync sırasında register'ların açıklama metnine dönüşerek kaybolduğu gerçek sunucuda kanıtlandı. HLL artık Redis HYLL binary string olarak saklanıyor; Redis hash/rank/kardinalite algoritmaları ve sparse/dense aktarım düzeltildi. 12 yeni diferansiyel vaka ile sabit corpus 393 vaka/391 birebir eşleşme ve iki belgeli kasıtlı farka ulaştı. 0–20.000 öğede tüm register'lar ve sayımlar karşılaştırıldı; iki yönlü SET/DUMP/RESTORE, AOF rewrite, snapshot, full/partial sync, PFCOUNT cache kaydı, readonly replica/Lua, AOF arızasında okuma, yarış ve fuzz doğrulandı. Açıklama metni dışında serileştiricisi olmayan string biçimli değerler kalıcılıkta reddediliyor. Eski kayıtta zaten kaybolan register'lar kurtarılamaz; ham PFADD geçmişi düzeltilmiş Redis algoritmasıyla okunur. Açık: uzantı veri modelleri, tüm yazıcıların bellek kabulü, toplam RAM/geçici tamponlar, yük/uzun çalışma, native platform/power-loss doğrulaması ve %100 kapsam.

GEOADD veri güvenliği (25 Ekim 2026): geçersiz çoklu isteğin kısmi veri bırakması, 4 KB noeviction bütçesinin 24 KB'ı aşan büyümeyi kabul etmesi ve uzun üye adlarının muhasebeye katılmaması çalıştırılabilir kanıtlarla doğrulandı. Tam koordinat/seçenek doğrulaması mutasyondan önce yapılıyor; nihai ayak izi kabul edilmeden harita yayınlanmıyor. Hata ve no-op yolları WATCH/AOF etkilerini koruyor; EXEC/Lua, eski AOF uyumluluğu ve metadata yeniden açılışı test edildi. Yeni corpus özel geo tipi/zset ve GEOPOS null biçimi farklarını üç açık uyumluluk hatası olarak görünür kılıyor; kasıtlı fark diye kapatılmadı. Açık: geo/zset veri modeli ve binary aktarım uyumluluğu, büyük geo indeksinde planlama/yayınlama maliyeti, diğer yazıcıların tam bellek kabulü ve genel production-ready çıkış ölçütleri.

Geo/zset birleştirmesi (25 Ekim 2026): GEOADD artık Redis geohash skorlarıyla normal sorted set yayınlıyor; tüm geo okuyucuları sorted set verisini çözüyor. GEOPOS null-array/koordinat hassasiyeti, GEOHASH, mesafe modeli/birimleri, dikdörtgen geometrisi ve STORE sonuçları düzeltildi. Sonuçlar AOF/replikasyona hesaplanmış RESTORE/DEL olarak gidiyor. Bulunamayan merkez üyesinin hedefi silmesi, GEORADIUS hedef ACL kontrolünün atlanması ve GEORADIUSBYMEMBER STORE disk hatasında başarı+MISCONF çift yanıtı çalıştırılarak kanıtlandı ve düzeltildi. Salt okunur radius varyantları eklendi; normal okumalar WATCH/journal değiştirmiyor. Sonlu eski native geo snapshot değerleri sorted set olarak yükleniyor; temsil edilemeyen tarihsel NaN verisi özel tipte korunuyor. Önceki üç geo verdict kapandı. Açık: büyük indeks tarama/kopyalama maliyeti, tüm sorgu sözdizimi/sıralama uçları, tüm mutasyonların bellek kabulü, toplam süreç RAM'i, fiziksel güç kesintisi/native platform testleri ve %100 test kapsamı.

Genel Set kabulü (25 Ekim 2026): Store.Set/GetOrCreate aynı atomik ayak izi rezervasyonuna geçirildi. Anahtar/entry/etiket maliyetleri dahil ediliyor; değiştirmede önceki muhasebeleştirilmiş değer kredileniyor, eşzamanlı DB yazmaları ortak bütçeyi ayırmadan yayınlayamıyor. Aynı boyutlu değiştirme OOM'u, uzun anahtar/etiketlerin sınırı aşması, kapılarla zorlanan iki-yazıcı taşması ve geçmiş KEEPTTL'nin yeni değeri silmesi düzeltme öncesi testte doğrulandı. NX/XX canlı entry kilidi altında değerlendiriliyor; metadata/expiry yayınla sıralanıyor ve giriş etiketi kopyalanıyor. Gerçek büyüme reddini ölçmeyen eski OOM kurulumları düzeltildi, hata/veri koruma kontrolleri korundu. Açık: yerinde koleksiyon/etiket büyümesi, SetEntry/taşıma yolları, DB'ler arası eviction seçimi, süreç RAM/geçici tampon bütçesi, uzun yük/native platform/güç kesintisi ve %100 kapsam hedefi.

Güncel kapsam ölçümü (25 Ekim 2026): son kaynakla `go test ./internal/... -coverprofile=... -count=1` geçti. Birleşik Go statement kapsamı %87,8; command %89,1, store %86,1, cluster %97,7, resp %88,3, persistence %80,5, server %90,8 ve replication %50,4. AGENTS.md'deki eski %16,3/%12,3/%23,3/%45,6 değerleri bu ölçümle güncellendi. Standart paket bazlı native Go ölçümüdür; `-coverpkg` ile çapraz paket ve diğer platform çalışma kapsamı ayrıca ölçülmelidir. %100 hedefi tamamlanmadı.

Entry aktarımı (25 Ekim 2026): SetEntry hata döndürüyor ve tam hedef ayak izini ayırıyor. RENAME/RENAMENX/MOVE/COPY artık kaynak/hedefi kararlı kilit sırasıyla birlikte tutuyor; eski hedef ve aynı bütçedeki taşınan kaynak kredileniyor, COPY klonu kabulden sonra oluşturuluyor. No-op ve COPY kaynak okuması WATCH/AOF'u değiştirmiyor; kaynak yalnızca başarılı kabulden sonra siliniyor. Okuyucu atomikliği için bu dört komut server gate'ini kalıcılık sonuna kadar tutuyor. COPY'nin büyük hedefi sessizce atıp başarı vermesi düzeltildi; eski kaydedilmiş no-op girdileri güvenilir replay'de okunuyor. Karşılıklı aktarım, ayrı/paylaşılan bütçeler, yükleme, metadata, EXEC/Lua/AOF ve süreç SIGKILL testleri eklendi. Açık: yerinde koleksiyon/etiket büyümesi, DB'ler arası eviction seçimi, büyük kopya duraklaması/geçici RAM ve genel production-ready çıkış ölçütleri.
