# player/
> L2 | 父级: ../../AGENTS.md

播放队列的唯一所有者与音频装载层：`Player` 在主线程序列化队列、播放、远程控制与会话状态，把网络、解码和目录查询放到 goroutine，再用 `post` 把结果带回主线。视图只读 `Current/Queue/Lyrics/Position` 等快照，不直接触碰流或下载。歌词加载见 `lyrics.go`：原音源优先，`SetLyricFallback` 默认关闭；`SetLyricConfig` 保存目录配置，开启时才按当前配置构建 NetEase/QQ/Kugou 链，配置变化即换链并重查。关闭后原音源照常、目录零调用、不建链；开启时补当前缺失曲目，配置变化取消旧查询并按新配置重查缺词或目录来源的曲目，原音源歌词、音频、队列与位置不动；`ClearLyricCache` 只清目录缓存。

## 成员清单
adts.go: AAC ADTS 解帧器，边下边找帧边界，并向 MyGo 音频层暴露可定位的 PCM 流。
ahead.go: 下一首音频的预取缓存，按 source/id/quality 命中，避免切歌时等待音源。
ahead_test.go: 预取命中、失效与质量匹配的测试。
cache.go: 磁盘歌曲缓存，按源/ID/质量命名，带容量上限与最久未用淘汰。
decoded.go: 把 `download` 适配成 `io.ReadSeeker`，交给 MyGo 的 MP4/FLAC/AAC 解码器，并统一 `stream` 接口。
download.go: `Song` 的查找与保存出口，复用缓存、磁盘文件或源站流，供离线/导出使用。
download_test.go: 歌曲查找、缓存复用与保存的测试。
fade.go: 暂停/恢复与切歌的音量淡入淡出器，叠加列表音量与 ReplayGain 增益。
fixture.go: 视图测试用的无装载装配入口：Cue/SetLyrics/Hold/Attach/SetStream；SetLyrics 取消未完成查询并把注入行标记为非目录来源，避免迟到结果覆盖。
formats_test.go: 各音频格式装载、定位与时长探测的测试。
lyrics.go: 歌词补全、目录配置与开关控制：原音源 LRC/逐字行优先；`SetLyricConfig` 保存配置并作废旧链，`SetLyricFallback` 开启时才按配置建链并查询，关闭时取消查询并移除目录行；进入 goroutine 前捕获配置、开关与装载代次，拒绝旧代与旧配置结果；`ClearLyricCache` 经可选 Clear 接口清目录缓存。
lyrics_test.go: 歌词优先、补全、元数据映射、旧结果丢弃、取消、补全总开关（保留原音源、关闭零目录调用并丢弃目录行、在途取消、开启补当前曲、重复设置不重查）、目录配置（关时零建链、开启按配置建链、配置变化取消旧查询并重查目录行、原音源保留、同配置无操作）与清缓存（保留当前行、下次重查）的离线测试。
ogg.go: Ogg Vorbis 流解码，尾部取时长，跨页定位，边下边播。
player.go: Player 核心状态：队列、播放/暂停、seek、会话恢复与装载代 gen；持有歌词目录配置、来源与开关状态，具体控制交给 lyrics.go。
player_test.go: 队列、淡入淡出、seek、电台、缓存、跳过不可播曲目与洗牌顺序的测试。
remote.go: PlaybackRemote 源的播放控制与轮询，跟随远端切歌、暂停、seek、音量与曲终；切歌重置歌词来源后复用统一加载器。
remote_test.go: 远端播放、轮询、暂停、seek、音量、跟随与错误路径的测试。
starved.go: 播放流的饥饿计时器，超过阈值让 `Loading` 报告缓冲中。
starved_test.go: 饥饿判定与恢复的测试。
stream.go: 下载缓冲、HTTP 拉取、共享读取、进度与频谱分析，以及按类型打开 `stream` 的工厂。
stream_test.go: 下载缓冲、共享、定位、频谱与格式打开的测试。
wav.go: WAV PCM 流解码，支持 8–32 位整型/浮点与直接按帧定位。
testdata/: seek 定位测试用的多格式短音频样本（mp3/m4a/aac/flac/ogg）。

[PROTOCOL]: 变更时更新此头部，然后检查 AGENTS.md
