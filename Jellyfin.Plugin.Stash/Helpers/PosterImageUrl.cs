using System;
using System.Collections.Generic;

namespace Stash.Helpers
{
    /// <summary>
    /// Marks a poster for proxy-side automatic cropping, without changing other
    /// query values (which may contain authentication or cache-version data).
    /// The marker does not select a crop position or perform image processing.
    /// </summary>
    public static class PosterImageUrl
    {
        public const string MarkerName = "emby_poster";

        public static string Create(string url, bool enabled)
        {
            if (!enabled || string.IsNullOrWhiteSpace(url))
            {
                return url;
            }

            if (!Uri.TryCreate(url, UriKind.Absolute, out var uri)
                || (uri.Scheme != Uri.UriSchemeHttp && uri.Scheme != Uri.UriSchemeHttps))
            {
                return url;
            }

            // Work with the original text rather than serializing UriBuilder:
            // unrelated query parameters must retain their exact encoding.
            var fragmentIndex = url.IndexOf('#');
            var fragment = fragmentIndex < 0 ? string.Empty : url.Substring(fragmentIndex);
            var resource = fragmentIndex < 0 ? url : url.Substring(0, fragmentIndex);
            var queryIndex = resource.IndexOf('?');
            var path = queryIndex < 0 ? resource : resource.Substring(0, queryIndex);
            var query = queryIndex < 0 ? string.Empty : resource.Substring(queryIndex + 1);
            var parameters = new List<string>();

            foreach (var parameter in query.Split('&'))
            {
                if (parameter.Length == 0)
                {
                    continue;
                }

                var equalsIndex = parameter.IndexOf('=');
                var key = equalsIndex < 0 ? parameter : parameter.Substring(0, equalsIndex);
                if (!string.Equals(Uri.UnescapeDataString(key), MarkerName, StringComparison.OrdinalIgnoreCase))
                {
                    parameters.Add(parameter);
                }
            }

            parameters.Add(MarkerName + "=1");
            return path + "?" + string.Join("&", parameters) + fragment;
        }
    }
}
