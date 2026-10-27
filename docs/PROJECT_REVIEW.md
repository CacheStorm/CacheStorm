# CacheStorm proje incelemesi

20 Ekim 2026 tarihinde `fe598ff` üzerindeki çalışma ağacı incelendi. Projede 628 takip edilen dosya, 18 internal paket, üç SDK, beş bağımsız Go örnek modülü ve bir React/Vite sitesi bulunuyor. İnceleme başlarken 143 dosyada mevcut değişiklik veya takip edilmeyen içerik vardı; bunlar korundu.

Öncelikli eksik, komut sayısı veya birim test yüzdesi değil, parçaların sunucu üzerinde aynı garantileri vermesidir. Testler başarılı olduğu halde AOF transaction kaydı, EXEC komut desteği, cluster/replika bağlantıları ve SDK davranışları için sekiz açık sorun yeniden üretildi. İki ek bakım sorunu bu çalışmada düzeltildi. Uygulama sırası [yol haritasında](../ROADMAP.md).

## 25 Ekim 2026 güncellemesi: üretime hazırlık çalışması

20 Ekim incelemesindeki F20 ve F22–F27 düzeltildi ve kalıcı testlere alındı; F21'in replikasyon tarafı uygulandı, cluster açıkça desteklenmiyor. Bu çalışma sırasında Redis 7.4'e karşı farksal bir uyumluluk düzeneği (`tests/compat`) kuruldu ve aşağıdaki ek sorunlar **çalıştırılarak kanıtlandı, ardından düzeltildi**. Her biri için regresyon testi var.

| Bulgu | Etki | Kanıt ve düzeltme |
|---|---|---|
| Komut adları büyük/küçük harfe duyarlı | go-redis, ioredis gibi küçük harf gönderen istemciler hiçbir komut çalıştıramıyordu | Router ve kayıt noktası adı normalize ediyor |
| RESP3 null (`_`) ve map (`%`) RESP2 istemcilere gönderiliyordu | 150 handler'ın null yanıtı RESP2 istemcilerde protokol hatası | Writer varsayılan olarak RESP2 kodluyor |
| Pub/Sub mesajları aboneye hiç ulaşmıyordu | PUBLISH alıcı sayıyor ama hiçbir istemci mesaj almıyordu | Bağlantı başına teslim goroutine'i, taşma halinde bağlantı kesme |
| İç içe dizi girdisi stack overflow ile süreci çökertiyordu | Kimliği doğrulanmamış herhangi bir istemci sunucuyu düşürebiliyordu | Komut okuma düz ve sınırlı; satır 64 KiB, iç içelik 128 ile sınırlı |
| Eşzamanlı push/pop'ta liste elemanları kayboluyordu | 20.000 push'ta %1–4 kayıp; hash/set/zset'te aynı desen | Boşalan değer yalnızca hâlâ aynı ve boşsa siliniyor; ilk oluşturma atomik |
| `maxmemory` yerinde büyüyen koleksiyonları saymıyordu | 1 MB limitle tek listeye 10 MB yazıldı, takip edilen ~1 KB | Koleksiyon boyutu artımlı tutuluyor, her yazmada yeniden hesaplanıyor, `denyoom` komutları OOM alıyor |
| EXEC ve Lua script'leri izole değildi | Okuyucu, 1.000 transaction'da 700 kez yarım uygulanmış durum gördü | Her komut yazma kapısını paylaşımlı, EXEC/script özel alıyor |
| SELECT sessizce OK dönüyordu | 16 veritabanı tek keyspace paylaşıyordu, `FLUSHDB` diğerlerini siliyordu | Gerçek veritabanları, AOF'ta SELECT, veritabanı başına WATCH |
| `BLPOP key 0` hemen dönüyor, uyandırma kaybolabiliyordu | Kuyruk işçileri boşa dönüyor ya da bir işi süresiz kaçırıyordu | Redis zaman aşımı anlamı, kayıttan sonra yeniden kontrol, FIFO sıra, kopan istemcinin bırakılması |
| LPUSH her çağrıda listeyi kopyalıyordu | LPUSH saniyede ~1.600 istek (diğer komutlar ~55.000) | Başta boşluk bırakan amorti O(1) ekleme |
| Varsayılan kurulum ağda parolasız açıktı | RESP, HTTP API ve pprof tüm arayüzlerde kimliksiz | Protected mode; parolasızken yalnızca loopback |
| Auth açıkken PING ve COMMAND kimliksiz çalışıyordu | Parolasız istemci sunucuyu yoklayabiliyordu | Yalnızca Redis'in `no_auth` komutları muaf |
| Yönetim arayüzü sürümsüz CDN script'leri yüklüyordu | CDN değişikliği parolayı tutan konsolda kod çalıştırabilirdi | Sürüm sabitlendi ve SRI eklendi |
| Bozuk AOF ile "boş başla" | Veri kaybolmuş gibi görünüyor, dosyanın arkasına yazılmaya devam ediliyordu | Yarım kuyruk kırpılıyor, bozuk dosya başlangıcı durduruyor |
| Kapanışta her boştaki istemci 10 sn bekletiyordu | Her kapanış en az 10 sn sürüyordu | Bağlantılar drain moduna alınıp hemen uyandırılıyor |

Uyumluluk ölçümü 307 vakada 138 eşleşmeden 346 vakada 345 eşleşmeye çıktı. Expiry artık milisaniye olarak saklanıyor; 2262 sınırı kaldırıldı ve AOF/RDB roundtrip testleriyle doğrulandı. go-redis v9, redis-py 5 ve ioredis 5 parola korumalı sunucuya karşı CI'da çalışıyor. Ölçülen performans [BENCHMARKS.md](BENCHMARKS.md) içinde.

Bilinen sınırlar: cluster modu yok; tam senkron fork'suz anlık görüntü alırken yazmaları duraklatır; bellek muhasebesi Go heap'i değil eleman tahminlerini sayar; otomatik failover gerçek süreçlerle doğrulanmadı; TypeScript ve Python paketleri yayımlanmadı.

## Kapsam ve yöntem

Tüm takip edilen dosyalar yapı, dosya türü, kullanım referansları ve bakım rolü bakımından envantere alındı. Sunucu başlangıç/kapanışı, router, transaction, RESP, store/TTL/eviction/tag, persistence, cluster, replication, Sentinel, ACL/HTTP, plugin/module, SDK, test, CI/release, config/installer ve web dokümantasyonu incelendi. Kritik davranışlara mevcut araçlarla küçük, ağdan bağımsız kanıtlar yazıldı.

Bu çalışma yaklaşık 199 bin satır takip edilen Go kodunun her satırının doğrulandığı bir güvence değildir. Çalıştırılmış bulgular aşağıda; kaynak okumaya dayalı bakım gözlemleri ayrı tutulmuştur. Yeni güvenlik açığı, performans regresyonu veya Redis uyumluluk yüzdesi ölçüldüğü iddia edilmiyor.

## Ölçülen durum

| Kontrol | Sonuç |
|---------|-------|
| `go test ./... -count=1 -timeout=180s -coverprofile=...` | Başarılı; atlanan entegrasyon senaryoları için tam destek anlamına gelmez |
| `go test -race ./internal/... -count=1 -timeout=180s` | Başarılı |
| `go build ./...` ve `go vet ./...` | Başarılı |
| `go mod verify` | Başarılı |
| `website/` içinde `npm run build` | Başarılı; Vite yapılandırması için ileriye dönük bir uyarı var |
| Yerel `golangci-lint run --timeout=5m` | Başarısız: üç staticcheck bulgusu; yerel sürüm 2.15.0, CI sürümü 2.13.2 |
| Üretim router'ı | 1.596 benzersiz komut kaydı, mükerrer kayıt yok |
| 18 internal paketin ağırlıklı statement kapsamı | %90,8 |
| Tüm instrument edilen kök modülün statement kapsamı | %88,2; SDK ve CLI'ın test edilmemiş kodu da dahil |
| Beş Go örneğinin readonly build'i | Başarısız; modül metadata'sı güncelleme istiyor |
| TypeScript SDK typecheck | Doğrulanamadı; SDK bağımlılıkları/Node tipleri bu çalışma ortamında kurulu değil |

Örneklerin ilk denemesi yerel Go 1.26.8 nedeniyle sürüm engeline takıldı. Ana modülün gerektirdiği `GOTOOLCHAIN=go1.26.9` ile tekrar denendiğinde beşi de `updates to go.mod needed, disabled by -mod=readonly` hatası verdi. Bu sonuç, örnek kaynaklarının tamamının derleme hatalı olduğu anlamına gelmez; temiz readonly build'in hazır olmadığını gösterir.

Tam kapsam tablosu [COVERAGE_REPORT.md](../COVERAGE_REPORT.md) içinde. Mevcut regresyonlar dahil çalışma ağacı ölçüldü; bu sayılar geçmiş sürümlere taşınamaz.

Yerel lint sonuçları: `internal/command/encoding_commands.go:233` için QF1002, `internal/command/sentinel_ismasterdown_test.go:79` için QF1001 ve `internal/server/connection.go:94` için QF1008. Üçü de inceleme başında değişiklik içeren dosyalarda; içerikleri korunarak raporlandı.

## Çalıştırılarak doğrulanan sorunlar

### F20 Transaction yazmaları yeniden yüklemede kayboluyor

**Yüksek öncelik, düzeltildi (24 Ekim 2026).** AOF `always` modundayken normal SET ve MULTI/SET/EXEC çalıştırıldı. EXEC başarı döndü ve transaction anahtarı bellekte oluştu. Aynı AOF yeni bir sunucuya yüklendiğinde normal SET geri geldi, transaction anahtarı gelmedi.

Sebep: [Router.Execute](../internal/command/router.go) kuyruklanan komut için persistence hook'una ulaşmıyor. [cmdEXEC](../internal/command/transaction_commands.go) ayrı dispatcher kullanıyor; [sunucunun writeCommands kapısı](../internal/server/server.go) EXEC'i kaydetmiyor. Başarılı transaction'ın veri güvenliği garantisi eksik.

### F21 Cluster ve replika ayarları çalışma durumuna bağlanmıyor

**Yüksek öncelik, replikasyon tarafı düzeltildi (25 Ekim 2026); cluster desteklenmiyor.** Replikasyonun tek sahibi artık sunucu başına `internal/replication.Manager`; komut katmanındaki taklit yönetici silindi. `replication.*` ayarları ve `REPLICAOF` gerçek PSYNC tam/kısmi senkronizasyon, salt-okunur replika, WAIT ve INFO replication sağlıyor; iki ve üç sunuculu uçtan uca testler `internal/server/replication_*_test.go` altında, iki süreçli doğrulama `scripts/replication-smoke.sh` içinde. Tam senkronizasyonda function kitaplıkları ve replikanın AOF kaydı da korunuyor; canlı kitaplık ekleme/silme ve SIGKILL sonrası AOF kurtarması aynı süreç testinde doğrulandı. Function kayıtları artık sunucuya ait; aynı süreçteki sunucular birbirlerinin kitaplıklarını görmüyor. Snapshot ayrı veri alanında hazırlanıyor; parse/komut/AOF hatası önceki canlı keyspace ve function kayıtlarını koruyor. Yayım bellek sayaçlarını ve Store sahiplerini koruyor; WATCH/etiket/bekleyen okuyucu bildirimleri başarılı geçişte güncelleniyor. Canlı komut hatasında akış konumu ilerletilmiyor, bağlantı ve alt replikalar bırakılıp tam senkronizasyon isteniyor. Hata öncesi kısmen değişmiş canlı komutların geri alınması henüz doğrulanmış bir garanti değil. `cluster.enabled=true` artık başlangıçta açık hata veriyor ve CLUSTER komutları cluster desteği kapalı bir Redis gibi yanıt veriyor. Özgün bulgu: `cluster.enabled=true` ve `replication.role=replica` ile sunucu oluşturuldu. PING başarılı, router kayıtları sağlam; buna karşılık komut katmanındaki rol `master`, CLUSTER HEALTH sonucu `ERR cluster not initialized`.

[Server.New/Start](../internal/server/server.go), [command replication manager](../internal/command/replication_commands.go), ayrı [replication.Manager](../internal/replication/replication.go) ve [InitCluster](../internal/command/cluster_commands.go) birlikte incelendi: production başlangıcı cluster/gossip veya ayrı replication manager yaşam döngüsünü bağlamıyor. Deterministik kanıt New sonrasındaki durumu gösteriyor; Start'taki bağlantı eksikliği kaynakta kontrol edildi. Gerçek ağda failover testi yapılmadı.

### F22 Desteklenen komut EXEC içinde çalışmıyor

**Yüksek öncelik, düzeltildi (24 Ekim 2026).** HGETALL tek başına beklenen field/value sonucunu verdi. Aynı komut MULTI içinde QUEUED kabul edildi; EXEC çalışınca `ERR command not supported in transaction` döndü.

[executeQueuedCommand](../internal/command/transaction_commands.go) kayıtlı handler'lar yerine kısmi bir switch tutuyor. Normal komut yürütme, transaction ve scripting farklı davranış kaynaklarına sahip. Komut paritesi ayrıca sınanmalı.

### F23 Python codec binary veriyi ve UTF-8 uzunluklarını bozuyor

**Yüksek öncelik, düzeltildi (24 Ekim 2026).** ASCII kontrolü geçti. `ş` iki byte taşırken RESP başlığı bir byte bildirdi. `ff00` binary girdisi yeniden encode edilirken `efbfbd00` oldu.

[encode_resp](../clients/python/cachestorm/protocol.py) karakter sayısını byte uzunluğu yerine kullanıyor ve binary veriyi UTF-8 replacement ile dönüştürüyor. Türkçe metin, emoji veya genel binary veri için protokol ve içerik korunmuyor.

### F24 TypeScript pipeline komut göndermeden başarı bildiriyor

**Yüksek öncelik, düzeltildi (24 Ekim 2026).** Aynı istemcinin normal GET çağrısı fake transport'a yazıp değer döndürdü. Pipeline SET/GET ise transport'a sıfır kez yazdı ve `['OK', 'value']` yerine `['OK', 'OK']` döndürdü.

[Pipeline.exec](../clients/typescript/src/pipeline.ts) doğrudan `Promise.resolve('OK')` üretiyor. Sonuç, gönderilen işlemin başarısını temsil etmiyor.

### F25 TypeScript decoder TCP parçalarını RESP mesajı sanıyor

**Yüksek öncelik, düzeltildi (24 Ekim 2026).** Tek parçalı `hello` yanıtı doğru okundu. Aynı yanıt iki parçaya ayrılınca `he` tamamlanmış sonuç sayıldı. `:42` sayı yerine metin kaldı; hata yanıtı bekleyen isteği tamamlamadı. UTF-8 encode uzunluğu da yanlış.

[handleResponse ve encodeResp](../clients/typescript/src/client.ts) birikimli framing ve byte tabanlı codec yerine string işlemleri kullanıyor. Sorun ağ zamanlamasından bağımsız olarak doğrudan callback girdileriyle gösterildi.

### F26 Python async istemcisi varsayılan haliyle bağlanamıyor

**Yüksek öncelik, düzeltildi (24 Ekim 2026).** Ağ kullanmayan fake bağlantı üzerinde normal constructor sonrası `connect`, `socket_connect_timeout` alanı yok hatası verdi. Eksik alan enjekte edilen kontrol istemcisi aynı fake bağlantıyı kurabildi.

[AsyncCacheStormClient](../clients/python/cachestorm/client.py) bağlantı fonksiyonunda constructor'ın tanımlamadığı bir alanı okuyor.

### F27 Go istemcisi Close sonrasında komutta panikliyor

**Orta öncelik, düzeltildi (24 Ekim 2026).** Tekrarlanan Close kontrolü başarılı. Ardından Ping, herhangi bir soket açmadan nil pointer paniğine düştü.

[pool.Get ve Client.Do](../clients/go/client.go), kapatılmış kanaldan gelen nil bağlantıyı kontrol etmeden kullanıyor. Kapanmış istemci hatası dönmesi gerekirken süreç seviyesinde panik oluşabiliyor.

### F28 Ana Docker sağlık kontrolü yanlış adrese gidiyor

**Orta öncelik, düzeltildi.** Ana Dockerfile `/health` adresini çağırıyordu; gerçek HTTP handler bu isteğe 404, `/api/health` isteğine 200 verdi. URL [Dockerfile](../Dockerfile) içinde `/api/health` olarak değiştirildi. Aynı kanıt ve ek HTTP durum kontrolleriyle doğrulandı. Docker daemon üzerinde çalışan imaj testi yapılmadı.

### F29 Dokümanlar olmayan dosyalara bağlantı veriyor

**Düşük öncelik, 14 bağlantı düzeltildi.** Doküman ana sayfasında dokuz, başlangıç rehberinde iki, SDK README dosyalarında üç bulunmayan hedef vardı. İndeks gerçek belgelere yönlendirildi; SDK API bağlantıları kaynak uygulamalara bağlandı.

Önceden değiştirilmiş kök README'deki dört workflow bağlantısı korundu: `nightly.yml`, `changelog.yml`, `dependency-update.yml`, `docs.yml` depoda yok. Bunlar düzeltilmiş sayılmıyor. Aktif workflow'lar CI, Release ve Website.

Genişletilmiş bölüm bağlantısı kontrolü ayrıca bu üç korunmuş belgede 14 eski anchor hedefinin başlıklarla uyuşmadığını gösterdi. README'nin `#installation` bağlantısı ile komut belgelerindeki bazı içindekiler hedefleri bu grupta. Yeni/değiştirilen belgelerde yeni kırık dosya veya bölüm hedefi bırakılmadı.

## Paketler ve bakım eksikleri

| Alan | İncelemenin sonucu ve sonraki kontrol |
|------|--------------------------------------|
| Server, router, command | Çok geniş komut yüzeyi; ayrı transaction/script dispatch ve elle tutulan write/ACL listeleri davranış sapması oluşturuyor. Komut metadata'sı merkezileştirilmeli. |
| Store, TTL, eviction, tag | %94,0 kapsam ve yarış denetimi başarılı. Çoklu anahtar atomikliği, WATCH sürümleri, OOM sonrası davranış, tag/expiry muhasebesi için sözleşme testleri öncelikli. Yeni yarış hatası doğrulanmadı. |
| RESP ve buffer | Byte seviyesinde temel testler var. Mevcut kaynakta tam RESP3 tip kümesi bulunmuyor; parser sınırları ve framing için fuzz hedefleri eklenmeli. |
| Persistence | 25 Ekim: SAVE/BGSAVE, snapshot_interval ve native snapshot başlangıç yüklemesi sunucuya bağlandı. Snapshot çoklu DB, TTL, etiket, stream group/pending ve function kitaplıklarını koruyor; mevcut AOF öncelikli, eksik AOF snapshot ile oluşturuluyor. CRC64 ve atomik dosya değişimi eklendi. AOFWriter değişimi artık yazarı durdurmuyor; create/write/fsync/rename/directory-sync hataları, geri alma ve eşzamanlı append kalıcı testlerle doğrulandı. Geri alma başarısızlığında önceki veri korunuyor, kurtarma yolu bildiriliyor ve writer hata döndürüyor. AOF yazma/flush/fsync hataları artık kalıcı hata durumu oluşturuyor; komut/EXEC/script yanıtı MISCONF, HTTP yazması 503 oluyor; sonraki AOF kapsamındaki yazmalar uygulama öncesi reddediliyor. INFO/readiness hata durumunu yansıtıyor. İlk hata sonrası bellek etkisi geri alınmıyor ve async modda önceden onaylanan yazmalar geri çağrılamıyor. Replika checkpoint artık AOF boyutu/SHA-256 ve kaynakla eşleşiyor; kapanış flush/fsync/close hatasını döndürüyor, marker dosyası/dizin senkronize ediliyor ve başlangıç silmesini kalıcılaştırıyor. Yerel replika yazmaları devam geçmişini geçersiz kılıyor. AOF semantik replay hataları artık başlangıcı durduruyor; yarım transaction kuyruğu MULTI sınırından fsync ile kaldırılıyor ve sonraki onaylı yazmaların kaybı engelleniyor. Her byte kesimi ve gerçek süreç SIGKILL/restart senaryosu test ediliyor. Veri dizini artık New sırasında OS kilidiyle tek sahibe ayrılıyor; yol sabitleniyor, hata/tam kapanışta bırakılıyor ve eksik drain'de korunuyor. Eşzamanlı sahip, symlink, snapshot worker ve SIGKILL/restart süreç testleri eklendi. Açık: diğer OS native/ağ dosya sistemi semantiği, deterministik replay/expiry-eviction etkileşimi, süreç geneli uzantı kayıtları, max_aof_size, snapshot yakalama maliyeti, güç kesintisi, dosya sistemi/donanım hata modeli, Windows çalışma zamanı ve keyspace dışında kalan uzantı kayıtlarının kalıcılığı. |
| Cluster, replication, Sentinel | Paket testleri güçlü olsa da ana sunucu entegrasyonu eksik. İki ayrı replication yönetimi var. Bazı slot komutları sabit boş/0/OK döndürüyor; gerçek slot taşıma/routing doğrulanmalı. |
| ACL ve HTTP | ACL kapsamı %100; son değişikliklerde anahtar/kanal yetkisi regresyonları var. Config'teki HTTP rate ayarları HTTPConfig'e aktarılmıyor; rate limiter sabit 100/dakika kuruluyor. Yetki kontrollerinin tüm yeni komutlara uygulanması sürdürülmeli. |
| Config ve logger | `config/example.yaml` güncel struct alanlarıyla uyuşmayan eski şema taşıyor. Loader YAML içindeki bilinmeyen alanları sessizce kabul ediyor. Rehberlerdeki otomatik dosya arama ve CLI flag açıklamaları gerçek başlangıçla uzlaştırılmalı. |
| Pool ve batch | Paket testleri geçiyor. SDK'ların ayrı havuz uygulamaları var; timeout, max connection ve kapanış garantilerinin paket kapsamıyla karıştırılmaması gerekiyor. |
| Graph, search, module, plugin | Bağımsız birim testleri var. Sunucuya bağlanan komut, ACL, kalıcılık ve yaşam döngüsü kapsamı ayrıca ölçülmeli; paket test sonucu production özelliğinin tamamlandığını kanıtlamıyor. |
| SDK'lar | Go SDK coverage %0; Python/TypeScript için takip edilen otomatik test yok. Python paket manifesti yok. TypeScript test/lint script'leri jest/eslint çağırıyor ama bu araçlar manifestte tanımlı değil. Go/TypeScript Pub/Sub tamamlanmamış. |
| Test ve benchmark | `internal/command` yaklaşık 49 bin uygulama satırı ve çok sayıda coverage/audit testi içeriyor. Güçlü mevcut regresyonlar korunmalı; yalnızca adı coverage/audit diye test silinmemeli. Karşılaştırmalı Redis ve uçtan uca dayanıklılık testleri eklenmeli. |
| CI ve release | Ana kalite kapısı race için yalnızca internal paketleri çalıştırıyor; ayrı örnek modüller ve SDK'lar kapsam dışı. Website build workflow'u PR kapısı değil. Aktif release GHCR ve binary arşivler yayımlıyor; belgelerde daha fazla yayın kanalı vaat ediliyor. |
| Config, Docker ve installer | Üç Dockerfile farklı kullanımda. Cluster Compose'un CACHESTORM_CLUSTER_* değişkenleri config loader tarafından okunmuyor. Root Compose, önce oluşturulması gereken `config/cachestorm.yaml` dosyasını bekliyor. Konteyner startup smoke testi gerekli. |
| Monitoring | HTTP metrics auth gerektirebiliyor, Prometheus örneğinde auth yok. `alerts.yml` konfigürasyonda referanslı ama Compose yalnızca prometheus.yml dosyasını mount ediyor. Auth açık kurulum uçtan uca doğrulanmalı. |
| Website ve belgeler | Markdown rehberleri ile TSX sayfalar aynı bilgiyi ayrı tutuyor; komut sayısı, sürüm, lisans ve destek açıklamaları sapmış. Tek kaynak veya drift kontrolü gerekli. |

Tablodaki ek işler kaynak incelemesine dayalı bakım gözlemleridir; hepsi için çalıştırılmış hata kanıtı sunulduğu anlamına gelmez. Yarış, güvenlik veya performans kusuru olarak sınıflandırılmadılar.

## Belge doğruluğu

- Kayıtlı komut sayısı 1.596. Çeşitli belgelerde 1.578, 1.606 ve 1.600+ geçiyor; kayıt sayısı ile eksiksiz destek ayrı ölçülmeli.
- AGENTS.md eski kapsam yüzdeleri; README, önceki kapsam raporu ve CONTRIBUTING birbirinden farklı daha yüksek rakamlar taşıyordu. Kapsam raporu ve katkı rehberi yenilendi. Kullanıcı değişikliği içeren README ve komut referansları korunarak kalan iş plana yazıldı.
- Lisans MIT olarak belirlendi (27 Ekim 2026): kök LICENSE, README, CONTRIBUTING, web sitesi ve SDK paket metadata'sı aynı lisansı gösteriyor.
- “%99 Redis uyumu” için hedef Redis sürümü, komut matrisi ve otomatik karşılaştırma sonucu yok. Çoklu DB, cluster, transaction ve protokol alt kümesi belirtilmeden bu oran kullanılamaz.
- Aktif workflow'lar `.github/workflows/ci.yml`, `release.yml` ve `website.yml`. Nightly, Homebrew/Scoop, GPG imzası ve DEB/RPM/APK yayını mevcut workflow'un yaptıkları arasında değil.

## Dosya temizliği ve saklama kararları

| Karar | Dosyalar | Gerekçe |
|-------|----------|---------|
| Arşivlendi | `files/` içindeki 8 Markdown → `docs/archive/design/` | Eski üretim prompt'ları mevcut uygulama ve proje talimatı gibi görünüyordu. Yaklaşık 186 KB içerik byte düzeyinde korunarak tarihsel arşive taşındı. |
| Arşivlendi | `.goreleaser.yml` → `docs/archive/release/goreleaser.yml` | Aktif workflow native build matrisi kullanıyor; eski yapılandırmanın release akışında referansı yok. |
| Kaldırıldı | `internal/persistence/coverage_test.go.bak` | Go test runner'ın çalıştırmadığı, referansı olmayan yaklaşık 20 KB eski test yedeği. |
| Kaldırıldı | `website/src/assets/vite.svg`, `typescript.svg`, `website/public/icons.svg` | Kaynak/index referansları bulunmayan başlangıç şablonu varlıkları. Web derlemesiyle kontrol edildi. |
| Düzenlendi | `.gitignore` | Python cache, Node bağımlılıkları, sanal ortam ve yedek çıktıları; çelişen/tekrarlanan wrongstack kuralları sadeleştirildi. |
| Düzenlendi | `.dockerignore` | Yaklaşık 135 MB web bağımlılığı, web çıktıları, 134 MB mevcut denetim klasörü ve yerel agent durumu build bağlamına taşınmasın. Bu boyutlar inceleme anındaki yerel dizin ölçümüdür. |
| Güncellendi | `docs/README.md`, `CONTRIBUTING.md`, `COVERAGE_REPORT.md`, `examples/README.md`, SDK README'leri | Kırık bağlantılar, eski metrikler ve uygulanmamış özellik iddiaları azaltıldı; mevcut sınırlamalar görünür hale getirildi. |
| Korundu | `config/example.yaml` | Eski şema olsa da Makefile setup ve installer tarafından kullanılıyor. Referanslar geçerli şablona taşınmadan silinemez. |
| Korundu | `docker/Dockerfile`, `examples/docker/Dockerfile` | Compose dosyaları bunları gerçekten kullanıyor; benzer olmaları gereksiz olduklarını göstermiyor. |
| Korundu | `snapcraft.yaml` | Manuel paketleme tanımı olabilir; otomatik yayın workflow'u bulunmaması tek başına silme gerekçesi değil. |
| Korundu | `.claude/settings.local.json` | Takip edilen kişisel araç ayarı. Ignore kuralı eklendi ama izlenen dosyaya bu kural geriye dönük etki etmez; içerik silinmedi. Ayrı bir değişiklikte tracking kararı verilmeli. |
| Korundu | `.temp_files/`, mevcut Python cache ve tüm regresyon testleri | Önceki denetim kanıtları ve mevcut çalışma korunmalı. Cache'i ignore etmek yeterli; kullanıcı içeriği topluca silinmedi. |
| Korundu | `docs/commands.md` ve `commands-detailed.md` | Referans ve örnek rehberi farklı amaçlı. İçlerinde kullanıcı değişiklikleri var; birleştirme kararından önce üretilen komut envanterine geçilmeli. |

Taşınan dokuz dosyanın SHA-256 değerleri taşımadan önce ve sonra karşılaştırıldı. Silinen dört dosyanın aynı hash'e sahip yerel yedekleri tutuldu. İşlem kaydı `.temp_files/project_review_2026_10_20/cleanup_manifest.json` içinde.

## Kanıtları tekrar çalıştırma

Kanıtlar `.temp_files/` altında yerel olarak korunur; Git tarafından izlenmez. 24 Ekim 2026 düzeltmelerinden sonra F20 ve F22–F27 **0 ile çıkar**; F21 açık olduğu için 1 ile çıkmaya devam eder. Kalıcı regresyonlar `internal/server/aof_transaction_test.go`, `internal/command/exec_dispatch_test.go` ve SDK test klasörlerindedir. Bunlar normal test paketine bilerek başarısız test olarak eklenmedi. F28 ve F29 düzeltme sonrası 0 ile çıkar. Kalıcı regresyonlar, runtime düzeltmeleriyle birlikte eklenmeli.

```bash
go test -overlay .temp_files/project_review_2026_10_20/runtime_overlay.json ./internal/server -run '^TestProjectReviewAOFTransactionX$' -count=1 -v
go test -overlay .temp_files/project_review_2026_10_20/runtime_overlay.json ./internal/server -run '^TestProjectReviewDistributedWiringX$' -count=1 -v
go run .temp_files/prove_F22_project_transaction_dispatch.go
python3 -B .temp_files/prove_F23_project_python_encoding.py
node .temp_files/prove_F24_project_typescript_pipeline.cjs
node .temp_files/prove_F25_project_typescript_protocol.cjs
python3 -B .temp_files/prove_F26_project_python_async.py
go run .temp_files/prove_F27_project_go_sdk_close.go
go test -overlay .temp_files/project_review_2026_10_20/runtime_overlay.json ./internal/server -run '^TestProjectReviewDockerHealthX$' -count=1 -v
python3 -B .temp_files/prove_F29_project_documentation.py
```

TypeScript kanıtları mevcut `website/node_modules/typescript` ile kaynakları bellekte transpile eder; yeni bağımlılık kurulmadı. Go overlay yalnızca geçici test dosyaları ekler. Repository kaynak dosyaları bu mekanizmayla değiştirilmez.

## Sınırlar ve sonraki adım

Gerçek Redis karşılaştırması, çalışan Docker imajı, çok düğümlü failover, disk/crash fault injection, SDK paket yayını, Windows/FreeBSD çalıştırma ve yeni performans baseline'ı yapılmadı. TypeScript SDK'nın tam typecheck'i kurulu bağımlılık eksikliği nedeniyle doğrulanmadı. Yerel lint bulgularının CI'daki farklı sürümle aynı sonucu vereceği varsayılmadı.

Runtime kusurlarına geniş bir refactor uygulanmadı; istenen inceleme, roadmap ve güvenli depo bakımına odaklanıldı. İlk geliştirme işi transaction kalıcılığı ve EXEC paritesi olmalı. Güvenilirlik ölçütleri tamamlanana kadar yeni komut sayısını artırmak öncelik olmamalı.

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
