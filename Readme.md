Herramienta de sincronización de acciones CodeQL
Logo

Una herramienta para sincronizar la acción de CodeQL de GitHub.com con el servidor de GitHub Enterprise, incluida la copia del paquete de CodeQL. Esto permite que CodeQL Action funcione incluso si sus ejecutores de GitHub Enterprise Server o GitHub Actions no tienen acceso a Internet.

Estado de desarrollo: listo para uso en producción

Instalación
La herramienta de sincronización de CodeQL Action se puede descargar desde la página de lanzamientos de este repositorio.

Uso
La herramienta de sincronización se puede utilizar de dos formas diferentes.

Si tiene una máquina que puede acceder a GitHub.com y la instancia del servidor de GitHub Enterprise, simplemente siga los pasos en "Tengo una máquina que puede acceder tanto a GitHub.com como al servidor de GitHub Enterprise" .

Si su instancia del servidor de GitHub Enterprise está en una red completamente aislada donde ninguna máquina tiene acceso a GitHub.com y al servidor de GitHub Enterprise, siga los pasos en "No tengo una máquina que pueda acceder tanto a GitHub.com como a GitHub Enterprise Server en su lugar.

Tengo una máquina que puede acceder tanto a GitHub.com como a GitHub Enterprise Server.
Desde una máquina con acceso tanto a GitHub.com como a GitHub Enterprise Server, use el ./codeql-action-sync synccomando para copiar la acción de CodeQL y los paquetes.

Argumentos requeridos:

--destination-url - La URL de la instancia del servidor de GitHub Enterprise a la que enviar la acción.
--destination-token- Un token de acceso personal para la instancia de destino del servidor de GitHub Enterprise. Si el repositorio de destino está en una organización que aún no existe o de la que no eres propietario, tu token deberá tener el site_adminalcance para poder crear la organización o actualizar el repositorio en ella. La organización también se puede crear manualmente o se puede utilizar una organización existente de su propiedad, en cuyo caso los alcances repoy workflowson suficientes.
Argumentos opcionales:

--cache-dir- Un directorio temporal en el que almacenar los datos descargados de GitHub.com antes de que se carguen en el servidor de GitHub Enterprise. Si no se especifica, se utilizará un directorio junto a la herramienta de sincronización.
--source-token- Un token para acceder a la API de GitHub.com. Normalmente, esto no es necesario, pero se puede proporcionar si tiene problemas con la limitación de la tasa de API. El token no necesita tener ningún ámbito.
--destination-repository- El nombre del repositorio en el que crear o actualizar la acción CodeQL. Si no se especifica github/codeql-action, se utilizará.
--actions-admin-user- El nombre del usuario administrador de Actions, que se utilizará si está actualizando la acción CodeQL incluida. Si no se especifica actions-admin, se utilizará.
--force- De forma predeterminada, la herramienta no sobrescribirá los repositorios existentes. Proporcionar esta bandera lo permitirá.
--push-ssh- Envíe el contenido de Git a través de SSH en lugar de HTTPS. Para usar esta opción, debe tener configurado el acceso SSH a su instancia de GitHub Enterprise.
No tengo una máquina que pueda acceder tanto a GitHub.com como a GitHub Enterprise Server.
Desde una máquina con acceso a GitHub.com, use el ./codeql-action-sync pullcomando para descargar una copia de CodeQL Action y los paquetes en una carpeta local.

Argumentos opcionales:

--cache-dir- El directorio en el que se almacenan los datos descargados de GitHub.com. Si no se especifica, se utilizará un directorio junto a la herramienta de sincronización.
--source-token- Un token para acceder a la API de GitHub.com. Normalmente, esto no es necesario, pero se puede proporcionar si tiene problemas con la limitación de la tasa de API. El token no necesita tener ningún ámbito.
A continuación, copie la herramienta de sincronización y el directorio de caché en otra máquina que tenga acceso al servidor de GitHub Enterprise.

Ahora use el ./codeql-action-sync pushcomando para cargar la acción de CodeQL y los paquetes en el servidor de GitHub Enterprise.

Argumentos requeridos:

--destination-url - La URL de la instancia del servidor de GitHub Enterprise a la que enviar la acción.
--destination-token- Un token de acceso personal para la instancia de destino del servidor de GitHub Enterprise. Si el repositorio de destino está en una organización que aún no existe o de la que no eres propietario, tu token deberá tener el site_adminalcance para poder crear la organización o actualizar el repositorio en ella. La organización también se puede crear manualmente o se puede utilizar una organización existente de su propiedad, en cuyo caso los alcances repoy workflowson suficientes.
Argumentos opcionales:

--cache-dir - El directorio en el que se descargó previamente la acción.
--destination-repository- El nombre del repositorio en el que crear o actualizar la acción CodeQL. Si no se especifica github/codeql-action, se utilizará.
--actions-admin-user- El nombre del usuario administrador de Actions, que se utilizará si está actualizando la acción CodeQL incluida. Si no se especifica actions-admin, se utilizará.
--force- De forma predeterminada, la herramienta no sobrescribirá los repositorios existentes. Proporcionar esta bandera lo permitirá.
--push-ssh- Envíe el contenido de Git a través de SSH en lugar de HTTPS. Para usar esta opción, debe tener configurado el acceso SSH a su instancia de GitHub Enterprise.
Contribuyendo
Para obtener más detalles sobre la contribución de mejoras a esta herramienta, consulte nuestra guía para colaboradores .
