# CacheStorm proje incelemesi

20 Ekim 2026 tarihinde `fe598ff` üzerindeki çalışma ağacı incelendi. Projede 628 takip edilen dosya, 18 internal paket, üç SDK, beş bağımsız Go örnek modülü ve bir React/Vite sitesi bulunuyor. İnceleme başlarken 143 dosyada mevcut değişiklik veya takip edilmeyen içerik vardı; bunlar korundu.

Öncelikli eksik, komut sayısı veya birim test yüzdesi değil, parçaların sunucu üzerinde aynı garantileri vermesidir. Testler başarılı olduğu halde AOF transaction kaydı, EXEC komut desteği, cluster/replika bağlantıları ve SDK davranışları için sekiz açık sorun yeniden üretildi. İki ek bakım sorunu bu çalışmada düzeltildi. Uygulama sırası [yol haritasında](../ROADMAP.md).

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

**Yüksek öncelik, açık.** AOF `always` modundayken normal SET ve MULTI/SET/EXEC çalıştırıldı. EXEC başarı döndü ve transaction anahtarı bellekte oluştu. Aynı AOF yeni bir sunucuya yüklendiğinde normal SET geri geldi, transaction anahtarı gelmedi.

Sebep: [Router.Execute](../internal/command/router.go) kuyruklanan komut için persistence hook'una ulaşmıyor. [cmdEXEC](../internal/command/transaction_commands.go) ayrı dispatcher kullanıyor; [sunucunun writeCommands kapısı](../internal/server/server.go) EXEC'i kaydetmiyor. Başarılı transaction'ın veri güvenliği garantisi eksik.

### F21 Cluster ve replika ayarları çalışma durumuna bağlanmıyor

**Yüksek öncelik, açık.** `cluster.enabled=true` ve `replication.role=replica` ile sunucu oluşturuldu. PING başarılı, router kayıtları sağlam; buna karşılık komut katmanındaki rol `master`, CLUSTER HEALTH sonucu `ERR cluster not initialized`.

[Server.New/Start](../internal/server/server.go), [command replication manager](../internal/command/replication_commands.go), ayrı [replication.Manager](../internal/replication/replication.go) ve [InitCluster](../internal/command/cluster_commands.go) birlikte incelendi: production başlangıcı cluster/gossip veya ayrı replication manager yaşam döngüsünü bağlamıyor. Deterministik kanıt New sonrasındaki durumu gösteriyor; Start'taki bağlantı eksikliği kaynakta kontrol edildi. Gerçek ağda failover testi yapılmadı.

### F22 Desteklenen komut EXEC içinde çalışmıyor

**Yüksek öncelik, açık.** HGETALL tek başına beklenen field/value sonucunu verdi. Aynı komut MULTI içinde QUEUED kabul edildi; EXEC çalışınca `ERR command not supported in transaction` döndü.

[executeQueuedCommand](../internal/command/transaction_commands.go) kayıtlı handler'lar yerine kısmi bir switch tutuyor. Normal komut yürütme, transaction ve scripting farklı davranış kaynaklarına sahip. Komut paritesi ayrıca sınanmalı.

### F23 Python codec binary veriyi ve UTF-8 uzunluklarını bozuyor

**Yüksek öncelik, açık.** ASCII kontrolü geçti. `ş` iki byte taşırken RESP başlığı bir byte bildirdi. `ff00` binary girdisi yeniden encode edilirken `efbfbd00` oldu.

[encode_resp](../clients/python/cachestorm/protocol.py) karakter sayısını byte uzunluğu yerine kullanıyor ve binary veriyi UTF-8 replacement ile dönüştürüyor. Türkçe metin, emoji veya genel binary veri için protokol ve içerik korunmuyor.

### F24 TypeScript pipeline komut göndermeden başarı bildiriyor

**Yüksek öncelik, açık.** Aynı istemcinin normal GET çağrısı fake transport'a yazıp değer döndürdü. Pipeline SET/GET ise transport'a sıfır kez yazdı ve `['OK', 'value']` yerine `['OK', 'OK']` döndürdü.

[Pipeline.exec](../clients/typescript/src/pipeline.ts) doğrudan `Promise.resolve('OK')` üretiyor. Sonuç, gönderilen işlemin başarısını temsil etmiyor.

### F25 TypeScript decoder TCP parçalarını RESP mesajı sanıyor

**Yüksek öncelik, açık.** Tek parçalı `hello` yanıtı doğru okundu. Aynı yanıt iki parçaya ayrılınca `he` tamamlanmış sonuç sayıldı. `:42` sayı yerine metin kaldı; hata yanıtı bekleyen isteği tamamlamadı. UTF-8 encode uzunluğu da yanlış.

[handleResponse ve encodeResp](../clients/typescript/src/client.ts) birikimli framing ve byte tabanlı codec yerine string işlemleri kullanıyor. Sorun ağ zamanlamasından bağımsız olarak doğrudan callback girdileriyle gösterildi.

### F26 Python async istemcisi varsayılan haliyle bağlanamıyor

**Yüksek öncelik, açık.** Ağ kullanmayan fake bağlantı üzerinde normal constructor sonrası `connect`, `socket_connect_timeout` alanı yok hatası verdi. Eksik alan enjekte edilen kontrol istemcisi aynı fake bağlantıyı kurabildi.

[AsyncCacheStormClient](../clients/python/cachestorm/client.py) bağlantı fonksiyonunda constructor'ın tanımlamadığı bir alanı okuyor.

### F27 Go istemcisi Close sonrasında komutta panikliyor

**Orta öncelik, açık.** Tekrarlanan Close kontrolü başarılı. Ardından Ping, herhangi bir soket açmadan nil pointer paniğine düştü.

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
| Persistence | %87,2 kapsam; bazı rewrite/roundtrip testleri koşulsuz skip içeriyor. SAVE hook'u AOF Flush çağırıyor; RDB snapshot ile aynı garanti değil. Snapshot interval/max AOF ayarları ve startup restore kapsamı sunucuda tamamlanmalı. |
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
- Kök LICENSE Apache 2.0 metni içeriyor; README, website, TypeScript package metadata'sı ve arşivlenen release metadata'sında MIT yazıyor. Bu bir metin/metadata tutarsızlığı tespitidir; LICENSE değiştirilmedi. Yayın öncesi tek ve açık bir politika uygulanmalı.
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

Kanıtlar `.temp_files/` altında yerel olarak korunur; Git tarafından izlenmez. F20–F27 bilinen sorunları gösterdiği için **1 ile çıkmaları beklenir**. Bunlar normal test paketine bilerek başarısız test olarak eklenmedi. F28 ve F29 düzeltme sonrası 0 ile çıkar. Kalıcı regresyonlar, runtime düzeltmeleriyle birlikte eklenmeli.

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
