# internal/lyrics/source/
> L2 | 父级: ../AGENTS.md

自动歌词目录层：按配置指定的来源和顺序检索、匹配并取回 LRC。检索只发送曲目元数据，不带账号凭据；只有所有启用目录明确未命中才负缓存，临时故障与取消不作为缺失缓存。配置按值传入并在建链时固定，运行中的链不读取 UI 状态。

成员清单
source.go: 查询契约与 Chain 顺序调度；按配置的正/负缓存期限及容量缓存答案，支持零值禁用与 Clear 清除；用歌词解析器拒绝不可显示的 LRC。
config.go: Config 参数契约、默认参数、配置归一化和链构建；三个有序来源槽、整链/请求超时与缓存参数，明确保留空来源和零缓存。
config_test.go: 验证来源顺序、请求/整链超时、缓存到期、零值禁用、容量及清除缓存的实际查询行为。
match.go: 标题与歌手归一化；剥离 MV/remaster/feat 等装饰、取首位歌手、去标点并借 OpenCC t2s 统一繁简，供打分与缓存键使用。
concert_match.go: 版本与现场演唱会识别；从括号/破折号后缀解析 Live/Remix/Acoustic/Instrumental/Cover、年份与场馆残留，计算候选分层，避免录音室版与不同场次互相冒充。
netease.go: 网易云音乐 provider；cloudsearch 取候选（含歌手别名），song/lyric 取 LRC 与翻译，Referer 指向自建 Base。
qq.go: QQ 音乐 provider；client_search 取 songmid 与歌手别名，fcg_query_lyric 取 LRC 与翻译，Retcode 非零视为未命中。
kugou.go: 酷狗音乐 provider；search 返回 id+accesskey 并以 duration 参与检索，download 返回 base64 LRC，解码失败视为未命中。
source_test.go: 离线覆盖归一化、artist/duration/歧义门控、Chain 顺序/回落/正负缓存、繁简互查、取消优先、故障不负缓存与不可用 LRC 不阻断回落。
netease_test.go: 离线验证网易 JSON 形状、Referer、空歌词/坏 JSON/5xx 的错误语义。
qq_test.go: 离线验证 QQ songmid 取词、拒绝码未命中、HTML 响应为错误、繁体查询匹配简体候选。
kugou_test.go: 离线验证 id+accesskey 取词、base64 解码、无候选与坏下载的未命中。
concert_match_test.go: 离线回归陶喆 Soul Power 现场专辑与版本边界（录音室优先、跨年份/场馆拒绝、Live 作为普通词保留、片段拒绝）。
generalization_test.go: 离线覆盖周杰伦/五月天/Coldplay/Eason Chan 真实案例及负例（别名不可冒充主唱、封装词与场地括号、检索召回第 13 位）。

[PROTOCOL]: 变更时更新此头部，然后检查 AGENTS.md
