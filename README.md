# diff-docx

Word文書（.docx）間の差分を比較するCLIツールです。テキスト差分と画像差分の両方を検出します。

## 機能

- **Markdown差分**: docxをMarkdownに変換し、[delta](https://github.com/dandavison/delta) でシンタックスハイライト付きの差分を表示
- **画像比較**: 文書内の画像をコンテンツベースでマッチングし、PSNR（Peak Signal-to-Noise Ratio）で差異を検出
- **diff出力**: 差分結果を `diff/` ディレクトリにファイル出力（diff.md、差分画像、変更された元画像）
- **プログレスバー**: tqdm風の進捗インジケーターを表示
- **シングルバイナリ**: Go製の静的バイナリとして配布可能

## 前提条件

### 必須ツール

diff-docxの動作には以下の3つの外部ツールが必要です。事前にインストールし、PATHが通っていることを確認してください。

#### 1. markitdown

docxをMarkdownに変換するために使用します。

```bash
pip install markitdown
#または
uv tool install markitdown
```

> Python 3.10以上が必要です。

インストール確認:

```bash
markitdown --version
```

#### 2. delta

Markdown差分をシンタックスハイライト付きで表示するために使用します。

| OS | コマンド |
| - | - |
| Ubuntu/Debian | ```sudo apt install git-delta``` |
| macOS (Homebrew) | ```brew install git-delta``` |
| Arch Linux | ```sudo pacman -S git-delta``` |
| Nix | [nixos.org](https://search.nixos.org/packages?channel=unstable&show=delta&from=0&size=50&sort=relevance&type=packages&query=delta) |
| Windows (scoop) | ```scoop install delta``` |
| Windows (winget) | ```winget install dandavison.delta``` |

インストール確認:

```bash
delta --version
```

#### 3. ImageMagick

画像のPSNR比較および差分画像の生成に使用します。**`magick` コマンド（v7系）** が必要です。

| OS | リンク |
| - | - |
| Ubuntu/Debian/Arch Linux | [Linux Binary Release](https://imagemagick.org/script/download.php#gsc.tab=0:~:text=before%20utilizing%20ImageMagick.-,Linux%20Binary%20Release,-These%20are%20the) |
| macOS | [macOS Binary Release](https://imagemagick.org/script/download.php#gsc.tab=0:~:text=Perl%2C%20and%20others.-,macOS%20Binary%20Release,-We%20recommend%20Homebrew) |
| Nix | [nixos.org](https://search.nixos.org/packages?channel=unstable&show=imagemagick&from=0&size=50&sort=relevance&type=packages&query=imagemagick) |
| Windows | [Windows Binary Release](https://imagemagick.org/script/download.php#gsc.tab=0:~:text=an%20iOS%20application.-,Windows%20Binary%20Release,-ImageMagick%20runs%20on) |

インストール確認:

```bash
magick --version
```

> ImageMagick v6系では `magick` コマンドが存在しないため動作しません。v7以上をインストールしてください。

### 任意ツール

#### LibreOffice（ベクター画像比較用、`--convert-png=false` 時のみ必要）

デフォルトではベクター画像（`.wmf`, `.emf`, `.svg`）はImageMagickでPNGに変換してから比較するため、LibreOfficeは不要です。

`--convert-png=false` を指定した場合のみ、ベクター画像の比較にLibreOfficeが必要になります。LibreOfficeがインストールされていない場合、ベクター画像の比較はスキップされます。

| OS | コマンド |
| - | - |
| Ubuntu/Debian | ```sudo apt install libreoffice``` |
| macOS (Homebrew) | ```brew install libreoffice``` |
| Arch Linux | ```sudo pacman -S libreoffice``` |
| Nix | [nixos.org](https://search.nixos.org/packages?channel=unstable&show=libreoffice&from=0&size=50&sort=relevance&type=packages&query=libreoffice) |
| Windows | ```winget install TheDocumentFoundation.LibreOffice``` |

インストール確認:

```bash
libreoffice --version
```

### 依存チェック

すべての必須ツールがインストール済みか確認できます:

```bash
make check-deps
```

## インストール

Go 1.24以上が必要です。

```bash
git clone https://github.com/shioshosho/diff-docx.git
cd diff-docx

# ビルド
make build

# /usr/local/bin にインストール
sudo make install

# カスタムパスにインストール
make build && sudo make install PREFIX=/opt
```

`diff-docx` と `diff-docx`（シンボリックリンク）の両方のコマンドが使えるようになります。

アンインストール:
```bash
sudo make uninstall
```

## 使い方

```bash
diff-docx <older.docx> <newer.docx>
```

### オプション

| オプション | 説明 |
|---|---|
| `-h`, `--help` | ヘルプを表示 |
| `-v`, `--version` | バージョンを表示 |
| `--verbose` | 詳細出力（一致画像、スキップ画像、差分画像パスを表示） |
| `--convert-png` | ベクター画像（wmf/emf/svg）をImageMagickでPNGに変換してから比較（デフォルト: true）。`--convert-png=false` で無効化 |

### 実行例

```bash
diff-docx docs/older.docx docs/newer.docx
```

## 出力

### ターミナル出力

実行中はプログレスバーが表示され、完了後に以下のセクションが出力されます:

```
=== Markdown Diff ===

(delta によるシンタックスハイライト付き差分)
qを押すと差分表示を終了

=== Image Comparison ===

  [DIFF] image1.png <-> image1.png (PSNR: 0.854)
  [DEL]  image5.emf (only in first document)
  [ADD]  image6.png (only in second document)
  2 difference(s) found.

=== Output ===
  diff/diff.md
  diff/imgs/ (1 diff images)
  diff/imgs/original/older/
  diff/imgs/original/newer/
```

`--verbose` を付けると `[SAME]`（一致）や `[SKIP]`（スキップ）のラベルも表示されます。

### ファイル出力

カレントディレクトリに `diff/` ディレクトリが生成されます。

```
diff/
├── diff.md                          # Unified diff（```diff コードブロック形式）
└── imgs/
    ├── image1-image1.png            # ImageMagick による差分画像
    └── original/
        ├── older/
        │   └── image1.png           # 旧文書の変更画像
        └── newer/
            └── image1.png           # 新文書の変更画像
```

- **`diff/diff.md`**: ```diff ``` コードブロックで囲まれたdiff形式のMarkdown。Markdownビューアーでハイライト表示されます。
- **`diff/imgs/`**: 差異があった画像ペアの差分画像（ImageMagick compare出力）。
- **`diff/imgs/original/<docx名>/`**: 差異があった画像・片方にしか存在しない画像のオリジナルファイル。

### Markdownファイル出力

各docxから変換されたMarkdownファイルは、元のdocxと同じディレクトリに保存されます。

```
docs/
├── older.docx
├── older.md    ← 自動生成
├── newer.docx
└── newer.md    ← 自動生成
```

Markdownファイル内の画像パスは、カレントディレクトリからの相対パスで記述されます（例: `./docs/older/word/media/image1.png`）。これは表示用の仮想パスであり、実ファイルは存在しません。
もし中身を確認したい場合は以下に対応

- Linux/macOSの場合

```bash
cp older.docx older.zip
unzip older.zip -d older
```

- Windowsの場合
    1. older.docxをコピー
    2. コピーしたolder.docxの名前をolder.zipへ変更
    3. older.zipをolderディレクトリへ展開
        - older直下にwordディレクトリが来るように展開するよう注意

## 画像比較の仕組み

### ファイル名のズレを吸収するためのコンテンツベースマッチング

新しいdocxの途中で画像の挿入・削除があった場合、docx間で画像のインデックスがその分ずれてしまいます。そのケースに対応するため、3段階のマッチングを行います:

1. **完全一致検出**: 拡張子ごとにグループ化し、全ペアを比較して内容が完全に一致するものをマッチング
2. **順序ベースのペアリング**: ステップ1で一致しなかった画像を順序で対応付け、差分画像を生成
3. **存在検出**: 対応するペアがない画像を `[DEL]`/`[ADD]` として報告

### PSNR値の解釈

ImageMagickの `compare -metric PSNR` でチャンネルごとのPSNR値を取得し、最小値で判定します。

| PSNR | 意味 |
|---|---|
| inf | 完全に同一 |
| > 30 | ほぼ同一（人間には区別困難） |
| 20 - 30 | わずかな差異 |
| < 20 | 明確な差異 |
| < 1.0 | 大きな差異（検出閾値） |

PSNR < 1.0 のチャンネルがひとつでもあれば「差異あり」と判定されます。

### 対応画像形式

| 種別 | 拡張子 | 条件 |
|---|---|---|
| ラスター | `.png`, `.jpg`, `.jpeg`, `.bmp`, `.gif`, `.tiff`, `.tif`, `.webp` | 常に比較可能 |
| ベクター | `.wmf`, `.emf`, `.svg` | デフォルト: ImageMagickでPNG変換して比較。`--convert-png=false` 時は LibreOffice が必要 |

## 一時ファイル

処理中の中間ファイル（docx展開、画像マッチング用一時ディレクトリ等）はOSのtempディレクトリに作成され、処理完了後に自動削除されます。

## 関連リンク

- [markitdown](https://github.com/microsoft/markitdown) - Microsoft製のドキュメント→Markdown変換ツール
- [delta](https://github.com/dandavison/delta) - シンタックスハイライト付きdiffビューアー
- [ImageMagick](https://imagemagick.org/) - 画像処理スイート
