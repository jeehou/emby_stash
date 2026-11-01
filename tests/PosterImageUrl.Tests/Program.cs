using Stash.Helpers;

var cases = new (string Name, string? Input, bool Enabled, string? Expected)[]
{
    ("disabled preserves original", "http://proxy/scene/12/screenshot?t=1#preview", false,
        "http://proxy/scene/12/screenshot?t=1#preview"),
    ("disabled does not rewrite old marker", "http://proxy/scene/12/screenshot?emby_poster=0", false,
        "http://proxy/scene/12/screenshot?emby_poster=0"),
    ("no query", "http://proxy/scene/12/screenshot", true,
        "http://proxy/scene/12/screenshot?emby_poster=1"),
    ("timestamp and escaped credentials", "https://proxy/scene/12/screenshot?t=1700&apikey=a%2Bb%2F%3D&x=a+b", true,
        "https://proxy/scene/12/screenshot?t=1700&apikey=a%2Bb%2F%3D&x=a+b&emby_poster=1"),
    ("fragment stays last", "http://proxy/scene/12/screenshot?t=1#poster?size=2", true,
        "http://proxy/scene/12/screenshot?t=1&emby_poster=1#poster?size=2"),
    ("fragment without query", "http://proxy/scene/12/screenshot#poster", true,
        "http://proxy/scene/12/screenshot?emby_poster=1#poster"),
    ("replace duplicate encoded markers", "http://proxy/scene/12/screenshot?emby_poster=0&t=1&%65mby_poster=2&EMBY_POSTER=3", true,
        "http://proxy/scene/12/screenshot?t=1&emby_poster=1"),
    ("idempotent", "http://proxy/scene/12/screenshot?t=1&emby_poster=1", true,
        "http://proxy/scene/12/screenshot?t=1&emby_poster=1"),
    ("bare marker", "http://proxy/scene/12/screenshot?emby_poster&t=1", true,
        "http://proxy/scene/12/screenshot?t=1&emby_poster=1"),
    ("empty query", "http://proxy/scene/12/screenshot?", true,
        "http://proxy/scene/12/screenshot?emby_poster=1"),
    ("ipv6", "http://[::1]:18080/scene/12/screenshot?t=1", true,
        "http://[::1]:18080/scene/12/screenshot?t=1&emby_poster=1"),
    ("relative URL left alone", "/scene/12/screenshot", true, "/scene/12/screenshot"),
    ("unsupported scheme", "file:///tmp/poster.jpg", true, "file:///tmp/poster.jpg"),
    ("null", null, true, null),
    ("empty", "", true, ""),
};

foreach (var test in cases)
{
    var actual = PosterImageUrl.Create(test.Input!, test.Enabled);
    if (actual != test.Expected)
    {
        // Avoid printing URLs: real-world examples may contain credentials.
        throw new Exception($"FAIL: {test.Name}");
    }
    Console.WriteLine($"PASS: {test.Name}");
}
Console.WriteLine($"{cases.Length} URL contract tests passed.");
