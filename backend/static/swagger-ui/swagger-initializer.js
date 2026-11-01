window.onload = function () {
  window.ui = SwaggerUIBundle({
    url: "/swagger/swagger.json",
    dom_id: "#swagger-ui",
    deepLinking: true,
    persistAuthorization: true,
    presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
    layout: "StandaloneLayout",
  });
};
