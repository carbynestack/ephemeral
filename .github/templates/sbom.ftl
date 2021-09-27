<#--
  Copyright (c) 2021 - for information on the respective copyright owner
  see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.

  SPDX-License-Identifier: Apache-2.0
-->
<#function formatLicenses licenses>
    <#assign result = "
         <licenses>"/>
    <#list licenses as license>
        <#assign result = result + "
            <license>
               <name>" + license.name + "</name>">
        <#if (license.url!"Unnamed")?index_of('Unnamed') == -1>
          <#assign result = result + "
               <url>" + license.url + "</url>">
        </#if>
        <#assign result = result + "
            </license>"/>
    </#list>
    <#assign result = result + "
         </licenses>">
    <#return result>
</#function>
<#function formatDependency dependency>
  <#assign result =
"         <name>" + (dependency.name!dependency.groupId) + "</name>
         <groupId>" + dependency.groupId + "</groupId>
         <artifactId>" + dependency.artifactId + "</artifactId>
         <version>" + dependency.version + "</version>">
  <#if (dependency.url!"Unnamed")?index_of('Unnamed') == -1>
      <#assign result = result + "
         <projectUrl>"+ dependency.url + "</projectUrl>">
  </#if>
  <#assign result = result + formatLicenses(dependency.licenses)>
  <#return result>
</#function>
<attributionReport>
   <dependencies>
<#list dependencyMap as map>
    <#assign dependency = map.getKey()/>
      <dependency>
${formatDependency(dependency)}
      </dependency>
</#list>
   </dependencies>
</attributionReport>
