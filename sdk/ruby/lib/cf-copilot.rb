# frozen_string_literal: true

require 'copilot/protos/cloud_controller_pb'
require 'copilot/protos/cloud_controller_services_pb'
require 'pry'

module Cloudfoundry
  module Copilot
    class Client
      class PilotError < StandardError; end
      CHUNK_SIZE = 64 * 1024 # 64KB

      attr_reader :host, :port

      def initialize(host:, port:, client_ca_file:, client_key_file:, client_chain_file:, timeout: 5)
        @host = host
        @port = port
        @url = "#{host}:#{port}"
        @timeout = timeout
        @client_ca = File.open(client_ca_file).read
        @client_key = File.open(client_key_file).read
        @client_chain = File.open(client_chain_file).read
      end

      def health
        request = Api::HealthRequest.new
        service.health(request)
      end

      def upsert_route(guid:, host:, path: '', internal: false, vip: nil)
        if internal && vip.nil?
          raise Cloudfoundry::Copilot::Client::PilotError, "vip required for internal routes"
        end
        route = Api::Route.new(guid: guid, host: host, path: path, internal: internal, vip: vip)
        request = Api::UpsertRouteRequest.new(route: route)
        service.upsert_route(request)
      rescue GRPC::BadStatus => e
        raise Cloudfoundry::Copilot::Client::PilotError, "#{e.details} - #{e.metadata}"
      end

      def delete_route(guid:)
        request = Api::DeleteRouteRequest.new(guid: guid)
        service.delete_route(request)
      rescue GRPC::BadStatus => e
        raise Cloudfoundry::Copilot::Client::PilotError, "#{e.details} - #{e.metadata}"
      end

      def map_route(capi_process_guid:, route_guid:, route_weight:)
        route_mapping = Api::RouteMapping.new(capi_process_guid: capi_process_guid, route_guid: route_guid, route_weight: route_weight)
        request = Api::MapRouteRequest.new(route_mapping: route_mapping)
        service.map_route(request)
      rescue GRPC::BadStatus => e
        raise Cloudfoundry::Copilot::Client::PilotError, "#{e.details} - #{e.metadata}"
      end

      def unmap_route(capi_process_guid:, route_guid:, route_weight:)
        route_mapping = Api::RouteMapping.new(capi_process_guid: capi_process_guid, route_guid: route_guid, route_weight: route_weight)
        request = Api::UnmapRouteRequest.new(route_mapping: route_mapping)
        service.unmap_route(request)
      rescue GRPC::BadStatus => e
        raise Cloudfoundry::Copilot::Client::PilotError, "#{e.details} - #{e.metadata}"
      end

      def upsert_capi_diego_process_association(capi_process_guid:, diego_process_guids:)
        request = Api::UpsertCapiDiegoProcessAssociationRequest.new(
          capi_diego_process_association: {
            capi_process_guid: capi_process_guid,
            diego_process_guids: diego_process_guids
          }
        )

        service.upsert_capi_diego_process_association(request)
      rescue GRPC::BadStatus => e
        raise Cloudfoundry::Copilot::Client::PilotError, "#{e.details} - #{e.metadata}"
      end

      def delete_capi_diego_process_association(capi_process_guid:)
        request = Api::DeleteCapiDiegoProcessAssociationRequest.new(
          capi_process_guid: capi_process_guid
        )
        service.delete_capi_diego_process_association(request)
      rescue GRPC::BadStatus => e
        raise Cloudfoundry::Copilot::Client::PilotError, "#{e.details} - #{e.metadata}"
      end

      class BulkSyncEnum
        def initialize(routes, route_mappings, capi_diego_process_associations)
          @routes = routes
          @route_mappings = route_mappings
          @capi_diego_process_associations = capi_diego_process_associations
        end

        def each
          return enum_for(:each) unless block_given?
          request = Api::BulkSyncRequest.new(
            routes: @routes,
            route_mappings: @route_mappings,
            capi_diego_process_associations: @capi_diego_process_associations
          )

          all_bytes = request.to_proto

          current_byte = 0
          request_bytes_length = all_bytes.length
          while current_byte < request_bytes_length
            yield Api::BulkSyncRequestChunk.new(chunk: all_bytes[current_byte..current_byte + CHUNK_SIZE - 1].to_s)
            current_byte += CHUNK_SIZE
          end
          yield Api::BulkSyncRequestChunk.new(chunk: all_bytes[current_byte..request_bytes_length-1].to_s)
        end

      end

      def bulk_sync(routes:, route_mappings:, capi_diego_process_associations:)
        bs_enum = BulkSyncEnum.new(routes, route_mappings, capi_diego_process_associations)
        service.bulk_sync(bs_enum.each)
      rescue GRPC::BadStatus => e
        raise Cloudfoundry::Copilot::Client::PilotError, "#{e.details} - #{e.metadata}"
      end

      private

      def tls_credentials
        @tls_credentials ||= GRPC::Core::ChannelCredentials.new(@client_ca, @client_key, @client_chain)
      end

      def compression_options
        @compression_options ||= GRPC::Core::CompressionOptions.new(
          default_algorithm: :gzip,
          default_level: :low
        )
      end

      def service
        @service ||= Api::CloudControllerCopilot::Stub.new(@url, tls_credentials, channel_args: compression_options.to_hash, timeout: @timeout)
      end
    end
  end
end
