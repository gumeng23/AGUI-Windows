package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// buildAndroid turns a single #target android source file into an APK using the
// bundled native runtime AAR and the Android SDK build tools. No Android
// project files are created next to the user's script.
func buildAndroid(path, src string, info targetInfo, out string, opts buildOptions) {
	out, err := filepath.Abs(out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "输出路径无效：%v\n", err)
		os.Exit(1)
	}
	sdk := findAndroidSDK()
	if sdk == "" {
		fmt.Fprintln(os.Stderr, "错误：找不到 Android SDK。请设置 ANDROID_HOME，或把 SDK 放到 build/sdk。")
		os.Exit(1)
	}
	javaHome := findJavaHome()
	if javaHome == "" {
		fmt.Fprintln(os.Stderr, "错误：找不到 JDK。请设置 JAVA_HOME。")
		os.Exit(1)
	}
	androidJar := filepath.Join(sdk, "platforms", "android-35", "android.jar")
	aapt2 := filepath.Join(sdk, "build-tools", "35.0.0", "aapt2.exe")
	d8 := filepath.Join(sdk, "build-tools", "35.0.0", "d8.bat")
	zipalign := filepath.Join(sdk, "build-tools", "35.0.0", "zipalign.exe")
	apksigner := filepath.Join(sdk, "build-tools", "35.0.0", "apksigner.bat")
	if !exists(androidJar) || !exists(aapt2) {
		fmt.Fprintf(os.Stderr, "错误：Android SDK 不完整，需要 platforms/android-35 与 build-tools/35.0.0。\n")
		os.Exit(1)
	}
	aar := findAndroidAAR()
	if aar == "" {
		fmt.Fprintln(os.Stderr, "错误：找不到 Android 运行时 android/runtime/agui.aar。")
		fmt.Fprintln(os.Stderr, "请先运行：gomobile bind -target=android/arm64 -androidapi 23 -javapkg=com.agui.runtime -o android/runtime/agui.aar ./mobile")
		os.Exit(1)
	}

	stage, err := os.MkdirTemp(".", ".agui-android-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "无法创建构建目录：%v\n", err)
		os.Exit(1)
	}
	stage, err = filepath.Abs(stage)
	if err != nil {
		fmt.Fprintf(os.Stderr, "无法解析构建目录：%v\n", err)
		os.Exit(1)
	}
	if os.Getenv("AGUI_KEEP_STAGE") == "" {
		defer os.RemoveAll(stage)
	}
	for _, d := range []string{"assets", "res/values", "res/drawable", "java", "classes", "gen", "dex"} {
		mustMkdir(filepath.Join(stage, d))
	}
	writeFile(filepath.Join(stage, "assets", "app.ag"), []byte(src))
	writeFile(filepath.Join(stage, "res", "values", "strings.xml"),
		[]byte(`<?xml version="1.0" encoding="utf-8"?><resources><string name="app_name">`+
			xmlEscape(info.Name)+`</string></resources>`))

	iconPath := info.Icon
	if iconPath != "" && !filepath.IsAbs(iconPath) {
		iconPath = filepath.Join(filepath.Dir(path), iconPath)
	}
	if iconPath == "" {
		if p := filepath.Join("android", "runtime", "icon.png"); exists(p) {
			iconPath = p
		}
	}
	if iconPath != "" {
		raw, err := os.ReadFile(iconPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "无法读取图标：%v\n", err)
			os.Exit(1)
		}
		writeFile(filepath.Join(stage, "res", "drawable", "icon.png"), raw)
	} else {
		writeFile(filepath.Join(stage, "res", "drawable", "icon.png"), defaultIconPNG())
	}

	// User assets. Files keep their base name; directories are mounted by their
	// base name, matching the desktop --asset behavior.
	for _, a := range opts.assets {
		if err := copyAsset(a, filepath.Join(stage, "assets")); err != nil {
			fmt.Fprintf(os.Stderr, "复制资源失败：%v\n", err)
			os.Exit(1)
		}
	}

	pkg := info.Package
	manifest := androidManifest(pkg, info, opts)
	writeFile(filepath.Join(stage, "AndroidManifest.xml"), []byte(manifest))

	// Extract classes.jar and the arm64 native library from the AAR.
	classesJar := filepath.Join(stage, "classes.jar")
	if err := extractAAR(aar, classesJar, filepath.Join(stage, "libgojni.so")); err != nil {
		fmt.Fprintf(os.Stderr, "读取 Android 运行时失败：%v\n", err)
		os.Exit(1)
	}

	tmpl, err := os.ReadFile(filepath.Join("android", "runtime", "MainActivity.java.tmpl"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "找不到 Android 模板：%v\n", err)
		os.Exit(1)
	}
	java := strings.ReplaceAll(string(tmpl), "__PACKAGE__", pkg)
	pkgDir := filepath.Join(stage, "java", filepath.FromSlash(strings.ReplaceAll(pkg, ".", "/")))
	mustMkdir(pkgDir)
	writeFile(filepath.Join(pkgDir, "MainActivity.java"), []byte(java))

	runTool(javaHome, "", "aapt2", aapt2, "compile", "--dir", filepath.Join(stage, "res"), "-o", filepath.Join(stage, "res.zip"))
	runTool(javaHome, stage, "aapt2 link", aapt2, "link", "-o", filepath.Join(stage, "base.apk"),
		"-I", androidJar, "--manifest", filepath.Join(stage, "AndroidManifest.xml"),
		"--java", filepath.Join(stage, "gen"), "-A", filepath.Join(stage, "assets"),
		filepath.Join(stage, "res.zip"), "--auto-add-overlay")

	rJava := filepath.Join(stage, "gen", filepath.FromSlash(strings.ReplaceAll(pkg, ".", "/")), "R.java")
	javac := filepath.Join(javaHome, "bin", "javac.exe")
	if !exists(javac) {
		javac = filepath.Join(javaHome, "bin", "javac")
	}
	runTool(javaHome, stage, "javac", javac, "-encoding", "UTF-8", "-source", "8", "-target", "8",
		"-bootclasspath", androidJar, "-classpath", classesJar, "-d", filepath.Join(stage, "classes"),
		filepath.Join(pkgDir, "MainActivity.java"), rJava)

	jar := filepath.Join(javaHome, "bin", "jar.exe")
	if !exists(jar) {
		jar = filepath.Join(javaHome, "bin", "jar")
	}
	appJar := filepath.Join(stage, "app-classes.jar")
	runTool(javaHome, stage, "jar", jar, "--create", "--file", appJar, "-C", filepath.Join(stage, "classes"), ".")
	runTool(javaHome, stage, "d8", d8, "--lib", androidJar, "--min-api", "23", "--output", filepath.Join(stage, "dex"), appJar, classesJar)
	if opts.aab {
		writeAAB(stage, out, aapt2, androidJar)
	} else {
		writeAPK(stage, out, javaHome, zipalign, apksigner, aapt2, opts)
	}
	fmt.Printf("✓ 已生成 %s（%d 字节）\n", out, fileSize(out))
}

func androidManifest(pkg string, info targetInfo, opts buildOptions) string {
	var perms strings.Builder
	for _, p := range info.Permissions {
		perms.WriteString(`    <uses-permission android:name="` + xmlEscape(p) + `" />` + "\n")
	}
	return `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    package="` + pkg + `"
    android:versionCode="` + fmt.Sprint(versionCode(opts)) + `"
    android:versionName="` + xmlEscape(info.Version) + `">
    <uses-sdk android:minSdkVersion="23" android:targetSdkVersion="34" />
` + perms.String() + `    <application
        android:label="@string/app_name"
        android:icon="@drawable/icon"
        android:allowBackup="false"
        android:extractNativeLibs="true"
        android:theme="@android:style/Theme.Material.Light.NoActionBar">
        <activity
            android:name="` + pkg + `.MainActivity"
            android:label="@string/app_name"
            android:exported="true"
            android:screenOrientation="portrait"
            android:configChanges="orientation|screenSize|keyboardHidden">
            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER" />
            </intent-filter>
        </activity>
    </application>
</manifest>
`
}

func versionCode(opts buildOptions) int {
	if opts.versionCode > 0 {
		return opts.versionCode
	}
	return 1
}

func writeAPK(stage, out, javaHome, zipalign, apksigner, aapt2 string, opts buildOptions) {
	base := filepath.Join(stage, "base.apk")
	unsigned := filepath.Join(stage, "unsigned.apk")
	if err := repackAPK(base, unsigned, filepath.Join(stage, "dex", "classes.dex"), filepath.Join(stage, "libgojni.so")); err != nil {
		fmt.Fprintf(os.Stderr, "组装 APK 失败：%v\n", err)
		os.Exit(1)
	}
	aligned := filepath.Join(stage, "aligned.apk")
	runTool(javaHome, stage, "zipalign", zipalign, "-f", "4", unsigned, aligned)
	ks := opts.ks
	alias := opts.keyAlias
	ksPass := opts.ksPass
	keyPass := opts.keyPass
	if !opts.release {
		ks = filepath.Join("android", "runtime", "debug.keystore")
		alias = "androiddebugkey"
		ksPass, keyPass = "android", "android"
	} else {
		if ks == "" || ksPass == "" {
			fmt.Fprintln(os.Stderr, "release 构建需要 --ks 与 --ks-pass，请使用自己的签名密钥")
			os.Exit(1)
		}
		if alias == "" {
			alias = "agui"
		}
		if keyPass == "" {
			keyPass = ksPass
		}
	}
	ks, _ = filepath.Abs(ks)
	mustMkdir(filepath.Dir(ks))
	if !exists(ks) {
		if opts.release {
			fmt.Fprintf(os.Stderr, "签名密钥不存在：%s\n", ks)
			os.Exit(1)
		}
		keytool := filepath.Join(javaHome, "bin", "keytool.exe")
		if !exists(keytool) {
			keytool = filepath.Join(javaHome, "bin", "keytool")
		}
		dname := "CN=Android Debug,O=Android,C=US"
		runTool(javaHome, stage, "keytool", keytool, "-genkeypair", "-keystore", ks, "-alias", alias,
			"-storepass", ksPass, "-keypass", keyPass, "-dname", dname,
			"-keyalg", "RSA", "-keysize", "2048", "-validity", "10000")
	}
	runTool(javaHome, stage, "apksigner", apksigner, "sign", "--ks", ks, "--ks-pass", "pass:"+ksPass,
		"--ks-key-alias", alias, "--key-pass", "pass:"+keyPass, "--out", out, aligned)
}

func repackAPK(base, out, dex, lib string) error {
	in, err := zip.OpenReader(base)
	if err != nil {
		return err
	}
	defer in.Close()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, f := range in.File {
		if f.Name == "classes.dex" || strings.HasPrefix(f.Name, "lib/") {
			continue
		}
		r, err := f.Open()
		if err != nil {
			return err
		}
		method := uint16(zip.Deflate)
		if f.Name == "resources.arsc" {
			// Android 11+ rejects compressed resources.arsc for targetSdk >= 30.
			method = zip.Store
		}
		h := &zip.FileHeader{Name: f.Name, Method: method}
		h.SetMode(f.Mode())
		ww, err := w.CreateHeader(h)
		if err != nil {
			r.Close()
			return err
		}
		if _, err := io.Copy(ww, r); err != nil {
			r.Close()
			return err
		}
		r.Close()
	}
	add := func(name, file string) error {
		r, err := os.Open(file)
		if err != nil {
			return err
		}
		defer r.Close()
		ww, err := w.Create(name)
		if err != nil {
			return err
		}
		_, err = io.Copy(ww, r)
		return err
	}
	if err := add("classes.dex", dex); err != nil {
		return err
	}
	if err := add("lib/arm64-v8a/libgojni.so", lib); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return os.WriteFile(out, buf.Bytes(), 0o644)
}

func extractAAR(path, classesJar, lib string) error {
	r, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		if f.Name == "classes.jar" {
			if err := extractZipFile(f, classesJar); err != nil {
				return err
			}
		}
		if f.Name == "jni/arm64-v8a/libgojni.so" {
			if err := extractZipFile(f, lib); err != nil {
				return err
			}
		}
	}
	if !exists(classesJar) || !exists(lib) {
		return fmt.Errorf("AAR 中缺少 classes.jar 或 arm64-v8a/libgojni.so")
	}
	return nil
}

func extractZipFile(f *zip.File, dest string) error {
	r, err := f.Open()
	if err != nil {
		return err
	}
	defer r.Close()
	w, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer w.Close()
	_, err = io.Copy(w, r)
	return err
}

func copyAsset(src, assets string) error {
	st, err := os.Stat(src)
	if err != nil {
		return err
	}
	if st.IsDir() {
		dst := filepath.Join(assets, filepath.Base(filepath.Clean(src)))
		return filepath.Walk(src, func(p string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(src, p)
			target := filepath.Join(dst, rel)
			if fi.IsDir() {
				return os.MkdirAll(target, 0o755)
			}
			raw, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			return os.WriteFile(target, raw, 0o644)
		})
	}
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(assets, filepath.Base(src)), raw, 0o644)
}

func runTool(javaHome, dir, name, bin string, args ...string) {
	low := strings.ToLower(bin)
	if strings.HasSuffix(low, ".bat") || strings.HasSuffix(low, ".cmd") {
		args = append([]string{"/c", bin}, args...)
		bin = "cmd.exe"
	}
	cmd := exec.Command(bin, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(),
		"JAVA_HOME="+javaHome,
		"PATH="+filepath.Join(javaHome, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s 失败：%v\n%s\n", name, err, truncateString(out.String(), 4000))
		os.Exit(1)
	}
}

func findAndroidSDK() string {
	for _, k := range []string{"ANDROID_HOME", "ANDROID_SDK_ROOT"} {
		if v := os.Getenv(k); v != "" {
			if exists(filepath.Join(v, "platforms", "android-35", "android.jar")) {
				return v
			}
		}
	}
	candidates := []string{"build/sdk"}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "sdk"))
	}
	for _, c := range candidates {
		if exists(filepath.Join(c, "platforms", "android-35", "android.jar")) {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	return ""
}

func findJavaHome() string {
	if v := os.Getenv("JAVA_HOME"); v != "" && exists(filepath.Join(v, "bin", "javac.exe")) {
		return v
	}
	candidates := []string{
		`C:\Users\WANDA\AppData\Roaming\.minecraft\runtime\java-runtime-delta`,
	}
	for _, c := range candidates {
		if exists(filepath.Join(c, "bin", "javac.exe")) {
			return c
		}
	}
	return ""
}

func findAndroidAAR() string {
	candidates := []string{filepath.Join("android", "runtime", "agui.aar")}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "android", "runtime", "agui.aar"))
	}
	for _, c := range candidates {
		if exists(c) {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	return ""
}

func writeAAB(stage, out, aapt2, androidJar string) {
	proto := filepath.Join(stage, "base-proto.apk")
	runTool("", stage, "aapt2 link --proto-format", aapt2, "link", "-o", proto,
		"-I", androidJar, "--manifest", filepath.Join(stage, "AndroidManifest.xml"),
		"-A", filepath.Join(stage, "assets"), filepath.Join(stage, "res.zip"),
		"--auto-add-overlay", "--proto-format")

	zr, err := zip.OpenReader(proto)
	if err != nil {
		fmt.Fprintf(os.Stderr, "无法读取 proto APK：%v\n", err)
		os.Exit(1)
	}
	defer zr.Close()
	mustMkdir(filepath.Dir(out))
	f, err := os.Create(out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "无法创建 AAB：%v\n", err)
		os.Exit(1)
	}
	zw := zip.NewWriter(f)
	add := func(name string, data []byte) {
		w, err := zw.Create(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "无法写入 AAB：%v\n", err)
			os.Exit(1)
		}
		if _, err := w.Write(data); err != nil {
			fmt.Fprintf(os.Stderr, "无法写入 AAB：%v\n", err)
			os.Exit(1)
		}
	}
	// BundleConfig: bundletool.version = "1.17.2" (minimal validated shape).
	add("BundleConfig.pb", []byte{0x0a, 0x08, 0x0a, 0x06, '1', '.', '1', '7', '.', '2'})
	add("base/manifest/AndroidManifest.xml", readZipFile(zr.File, "AndroidManifest.xml"))
	add("base/resources.pb", readZipFile(zr.File, "resources.pb"))
	for _, zf := range zr.File {
		switch {
		case strings.HasPrefix(zf.Name, "res/"):
			add("base/"+zf.Name, readZipFile(zr.File, zf.Name))
		case strings.HasPrefix(zf.Name, "assets/"):
			add("base/"+zf.Name, readZipFile(zr.File, zf.Name))
		}
	}
	add("base/dex/classes.dex", mustReadFile(filepath.Join(stage, "dex", "classes.dex")))
	lib := filepath.Join(stage, "libgojni.so")
	if !exists(lib) {
		lib = filepath.Join(stage, "lib", "arm64-v8a", "libgojni.so")
	}
	add("base/lib/arm64-v8a/libgojni.so", mustReadFile(lib))
	if err := zw.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "无法关闭 AAB：%v\n", err)
		os.Exit(1)
	}
	if err := f.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "无法关闭 AAB：%v\n", err)
		os.Exit(1)
	}
}

func readZipFile(files []*zip.File, name string) []byte {
	for _, f := range files {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			fmt.Fprintf(os.Stderr, "无法读取 APK 条目 %s：%v\n", name, err)
			os.Exit(1)
		}
		defer rc.Close()
		b, err := io.ReadAll(rc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "无法读取 APK 条目 %s：%v\n", name, err)
			os.Exit(1)
		}
		return b
	}
	fmt.Fprintf(os.Stderr, "APK 缺少条目 %s\n", name)
	os.Exit(1)
	return nil
}

func mustReadFile(path string) []byte {
	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "无法读取 %s：%v\n", path, err)
		os.Exit(1)
	}
	return b
}

func defaultIconPNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 192, 192))
	for y := 0; y < 192; y++ {
		for x := 0; x < 192; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: uint8(220 - y/4), B: uint8(200 - y/3), A: 255})
		}
	}
	for y := 54; y < 138; y++ {
		for x := 54; x < 138; x++ {
			img.Set(x, y, color.RGBA{R: 236, G: 120, B: 96, A: 255})
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func mustMkdir(p string) {
	if err := os.MkdirAll(p, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "无法创建目录 %s：%v\n", p, err)
		os.Exit(1)
	}
}

func writeFile(p string, b []byte) {
	mustMkdir(filepath.Dir(p))
	if err := os.WriteFile(p, b, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "无法写入 %s：%v\n", p, err)
		os.Exit(1)
	}
}

func fileSize(p string) int64 {
	st, err := os.Stat(p)
	if err != nil {
		return 0
	}
	return st.Size()
}

func truncateString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
